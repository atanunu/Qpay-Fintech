package backup

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

// Wall-clock schedules run once per local date. A spring-forward missing time is
// skipped, and the first UTC occurrence wins for a repeated autumn wall time.
// Intervals are elapsed UTC durations anchored at StartsAt, not wall-clock cron.
func ValidateSchedule(v ScheduleSpec) error {
	if !v.Plan.Valid() || !slices.Contains([]string{"backup", "full", "diff", "incr", "verify"}, v.Operation) {
		return errors.New("choose an exact approved plan and supported operation")
	}
	if _, err := time.LoadLocation(v.Timezone); err != nil {
		return errors.New("choose a valid IANA timezone")
	}
	if v.StartsAt.IsZero() || v.StartsAt.Year() < 2020 || v.StartsAt.Year() > 2200 {
		return errors.New("valid schedule start required")
	}
	if v.EndsAt != nil && (!v.EndsAt.After(v.StartsAt) || v.EndsAt.Sub(v.StartsAt) > 100*366*24*time.Hour) {
		return errors.New("end must follow start within 100 years")
	}
	if v.Missed != "skip" && v.Missed != "catch_up_once" {
		return errors.New("choose skip or catch_up_once for missed runs")
	}
	switch v.Frequency {
	case "once":
	case "interval":
		if v.IntervalMinutes < 15 || v.IntervalMinutes > 525600 {
			return errors.New("interval must be between 15 minutes and one year")
		}
	case "daily", "weekly", "monthly":
		if _, _, e := wallParts(v.At); e != nil {
			return e
		}
		if v.Frequency == "weekly" {
			if len(v.Weekdays) < 1 || len(v.Weekdays) > 7 {
				return errors.New("select weekdays")
			}
			seen := map[int]bool{}
			for _, d := range v.Weekdays {
				if d < 0 || d > 6 || seen[d] {
					return errors.New("weekdays must be unique, Sunday 0 through Saturday 6")
				}
				seen[d] = true
			}
		}
		if v.Frequency == "monthly" && (v.MonthDay != -1 && (v.MonthDay < 1 || v.MonthDay > 31)) {
			return errors.New("choose day 1–31 or -1 for the last day")
		}
	default:
		return errors.New("unsupported frequency")
	}
	return nil
}
func wallParts(s string) (int, int, error) {
	if len(s) != 5 || s[2] != ':' {
		return 0, 0, errors.New("time must use HH:MM")
	}
	h, e := strconv.Atoi(s[:2])
	m, f := strconv.Atoi(s[3:])
	if e != nil || f != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, errors.New("invalid wall time")
	}
	return h, m, nil
}
func onDate(v ScheduleSpec, t time.Time) bool {
	if v.Frequency == "weekly" && !slices.Contains(v.Weekdays, int(t.Weekday())) {
		return false
	}
	if v.Frequency == "monthly" {
		day := v.MonthDay
		if day == -1 {
			day = time.Date(t.Year(), t.Month()+1, 0, 12, 0, 0, 0, t.Location()).Day()
		}
		if t.Day() != day {
			return false
		}
	}
	return true
}
func wallTime(date time.Time, h, m int) (time.Time, bool) {
	y, mo, d := date.Date()
	nominal := time.Date(y, mo, d, h, m, 0, 0, date.Location())
	// Search a bounded UTC window: handles non-hour transitions without relying on
	// time.Date's unspecified preference for an ambiguous local wall time.
	start := nominal.Add(-3 * time.Hour).UTC().Truncate(time.Minute)
	for i := 0; i <= 360; i++ {
		u := start.Add(time.Duration(i) * time.Minute)
		l := u.In(date.Location())
		if l.Year() == y && l.Month() == mo && l.Day() == d && l.Hour() == h && l.Minute() == m {
			return u, true
		}
	}
	return time.Time{}, false
}
func Next(v ScheduleSpec, after time.Time, count int) ([]time.Time, error) {
	if err := ValidateSchedule(v); err != nil {
		return nil, err
	}
	if count < 1 || count > 100 {
		return nil, errors.New("preview count must be 1–100")
	}
	out := []time.Time{}
	add := func(t time.Time) bool {
		if t.Before(v.StartsAt) || !t.After(after) {
			return false
		}
		if v.EndsAt != nil && t.After(*v.EndsAt) {
			return true
		}
		out = append(out, t.UTC())
		return len(out) == count
	}
	if v.Frequency == "once" {
		add(v.StartsAt)
		return out, nil
	}
	if v.Frequency == "interval" {
		step := time.Duration(v.IntervalMinutes) * time.Minute
		t := v.StartsAt
		if !after.Before(t) {
			t = t.Add((after.Sub(t)/step + 1) * step)
		}
		for i := 0; i < count; i++ {
			if add(t) {
				break
			}
			t = t.Add(step)
		}
		return out, nil
	}
	loc, _ := time.LoadLocation(v.Timezone)
	floor := after
	if floor.Before(v.StartsAt) {
		floor = v.StartsAt
	}
	d := floor.In(loc)
	date := time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, loc)
	h, m, _ := wallParts(v.At)
	for i := 0; i < 366*9; i++ {
		if onDate(v, date) {
			t, ok := wallTime(date, h, m)
			if ok && add(t) {
				break
			}
		}
		date = date.AddDate(0, 0, 1)
	}
	return out, nil
}

// Due returns at most one execution, preventing an outage from causing a backup
// storm. The cursor is advanced to now even if a skip policy intentionally omits work.
func Due(v ScheduleSpec, last, now time.Time) (time.Time, bool, error) {
	if last.IsZero() {
		last = v.StartsAt.Add(-time.Nanosecond)
	}
	if now.Before(last) {
		return time.Time{}, false, errors.New("clock moved backwards")
	}
	next, e := Next(v, last, 1)
	if e != nil || len(next) == 0 || next[0].After(now) {
		return time.Time{}, false, e
	}

	if v.Frequency == "once" {
		return next[0], v.Missed != "skip" || now.Sub(next[0]) <= 5*time.Minute, nil
	}
	// Catch up once with the most recent due UTC occurrence, never a fictional old capture.
	best := next[0]
	if v.Frequency == "interval" {
		step := time.Duration(v.IntervalMinutes) * time.Minute
		best = best.Add(now.Sub(best) / step * step)
		if v.EndsAt != nil && best.After(*v.EndsAt) {
			best = next[0].Add(v.EndsAt.Sub(next[0]) / step * step)
		}
	} else {
		since := now.AddDate(0, 0, -40)
		if since.Before(last) {
			since = last
		}
		times, e := Next(v, since, 100)
		if e != nil {
			return time.Time{}, false, e
		}
		for _, t := range times {
			if t.After(now) {
				break
			}
			best = t
		}
	}
	return best, v.Missed != "skip" || now.Sub(best) <= 5*time.Minute, nil
}
func OccurrenceID(schedule Ref, t time.Time) string {
	return "br_" + Digest([]byte(fmt.Sprintf("%s/%s", schedule.Key(), t.UTC().Format(time.RFC3339Nano))))[:40]
}
func ScheduleDescription(v ScheduleSpec) string {
	if v.Frequency == "interval" {
		return fmt.Sprintf("Every %d minutes, UTC elapsed time", v.IntervalMinutes)
	}
	return strings.TrimSpace(v.Frequency + " " + v.At + " " + v.Timezone)
}
