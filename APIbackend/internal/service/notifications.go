package service

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
)

type NotificationMeta struct {
	Audience       string `json:"audience"`
	Scope          string `json:"scope"`
	Classification string `json:"classification"`
	Guard          string `json:"guard"`
	Subject        string `json:"subject"`
	Amount         bool   `json:"amount"`
	Code           bool   `json:"code"`
	TTL            int    `json:"ttl"`
}

//go:generate python3 ../../scripts/sync_catalogue.py
//go:embed catalogue/emails.psv
var notificationCatalogue string
var NotificationMetadata = func() map[string]NotificationMeta {
	rows := strings.Split(strings.TrimSpace(notificationCatalogue), "\n")
	if len(rows) < 2 || rows[0] != "key|audience|scope|class|theme|guard|subject|body" {
		panic("invalid embedded notification catalogue")
	}
	out := map[string]NotificationMeta{}
	for _, line := range rows[1:] {
		v := strings.Split(strings.TrimSuffix(line, "\r"), "|")
		if len(v) != 8 {
			panic("invalid notification row")
		}
		if _, exists := out[v[0]]; exists {
			panic("duplicate notification key")
		}
		m := NotificationMeta{Audience: v[1], Scope: v[2], Classification: v[3], Guard: v[5], Subject: v[6], Amount: strings.Contains(v[7], "{{amount}}"), Code: strings.Contains(v[7], "{{code}}"), TTL: 86400}
		if m.Classification == "operational" {
			m.TTL = 3600
		}
		if m.Code {
			m.TTL = 600
		}
		out[v[0]] = m
	}
	return out
}()

func (s *Service) notify(ctx context.Context, tx *sql.Tx, owner, workflow, reference string, values map[string]any, secret bool) error {
	return s.notifyOccurrence(ctx, tx, owner, workflow, reference, reference, values, secret)
}
func (s *Service) notifyOccurrence(ctx context.Context, tx *sql.Tx, owner, workflow, reference, occurrence string, values map[string]any, secret bool) error {
	return s.notifyTo(ctx, tx, owner, workflow, reference, occurrence, values, secret, "")
}
func (s *Service) notifyTo(ctx context.Context, tx *sql.Tx, owner, workflow, reference, occurrence string, values map[string]any, secret bool, email string) error {
	meta, ok := NotificationMetadata[workflow]
	if !ok || meta.Scope != "core" {
		return errors.New("unknown or inactive notification workflow")
	}
	if meta.Code != secret {
		return errors.New("notification sensitivity mismatch")
	}
	u, e := s.user(ctx, tx, owner, false)
	if e != nil {
		return e
	}
	if email == "" {
		email = u.Email
	}
	id := stringID("notice_")
	now := s.Now().UTC().Truncate(time.Millisecond)
	expires := now.Add(time.Duration(meta.TTL) * time.Second)
	payload := map[string]any{"version": 1, "event_id": security.Random("evt_", 18), "notification_id": id, "recipient_ref": owner, "reference": reference, "audience": meta.Audience, "state": meta.Guard, "locale": "en-NG", "name": u.Name, "occurred_at": now.Format("2006-01-02T15:04:05.000Z"), "expires_at": expires.Format("2006-01-02T15:04:05.000Z")}
	if meta.Amount {
		v, ok := values["amount_minor"]
		if !ok {
			return errors.New("notification amount required")
		}
		payload["amount_minor"] = v
		payload["currency"] = "NGN"
	}
	if meta.Code {
		v, ok := values["code"]
		if !ok {
			return errors.New("notification code required")
		}
		payload["code"] = v
	}
	// Optional/campaign producers require their own consent and unsubscribe contract before activation.
	if meta.Classification == "optional" || meta.Classification == "marketing" {
		return errors.New("optional notification producer not activated")
	}
	encrypted, e := s.Config.Box.Seal(jsonText(payload), "notification:"+id)
	if e != nil {
		return e
	}
	return exec(tx, ctx, `INSERT INTO notification_intents(id,owner_id,workflow,reference,recipient_email,payload_enc,secret,expires_at,created_at,occurrence) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(owner_id,workflow,reference,occurrence) DO NOTHING`, id, owner, workflow, reference, email, encrypted, secret, expires, now, occurrence)
}
func (s *Service) Notifications(ctx context.Context, owner string, limit int, before string) (map[string]any, error) {
	if limit < 1 || limit > 100 {
		return nil, Invalid("invalid limit")
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT id,workflow,reference,created_at,read_at FROM notification_intents WHERE owner_id=$1 AND NOT secret AND ($2='' OR id<$2) ORDER BY id DESC LIMIT $3`, owner, before, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, workflow, reference string
		var created time.Time
		var read sql.NullTime
		if e = rows.Scan(&id, &workflow, &reference, &created, &read); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": id, "workflow": workflow, "subject": NotificationMetadata[workflow].Subject, "reference": reference, "created_at": created, "read": read.Valid})
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	var count int
	if e = s.DB.QueryRowContext(ctx, `SELECT count(*) FROM notification_intents WHERE owner_id=$1 AND NOT secret AND read_at IS NULL`, owner).Scan(&count); e != nil {
		return nil, e
	}
	return map[string]any{"items": out, "unread": count}, nil
}
func (s *Service) ReadNotification(ctx context.Context, owner, id string) error {
	result, e := s.DB.ExecContext(ctx, `UPDATE notification_intents SET read_at=coalesce(read_at,$3) WHERE id=$1 AND owner_id=$2 AND NOT secret`, id, owner, s.Now())
	if e != nil {
		return e
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return missing()
	}
	return nil
}
func (s *Service) Preferences(ctx context.Context, owner string) (map[string]bool, error) {
	var optional, marketing bool
	e := s.DB.QueryRowContext(ctx, `SELECT optional_email,marketing_email FROM preferences WHERE owner_id=$1`, owner).Scan(&optional, &marketing)
	return map[string]bool{"optional_email": optional, "marketing_email": marketing}, isMissing(e)
}
func (s *Service) SetPreferences(ctx context.Context, p Principal, optional, marketing bool) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		if _, e := s.activePrincipal(ctx, tx, p); e != nil {
			return e
		}
		if e := exec(tx, ctx, `UPDATE preferences SET optional_email=$2,marketing_email=$3,updated_at=$4 WHERE owner_id=$1`, p.User.ID, optional, marketing, s.Now()); e != nil {
			return e
		}
		return s.audit(ctx, tx, p.User.ID, "preferences.updated", p.User.ID, map[string]any{"optional_email": optional, "marketing_email": marketing})
	})
}

// NotificationPolicy validates the exact stored payload, not client-asserted business state.
func (s *Service) NotificationPolicy(ctx context.Context, id string, supplied json.RawMessage) (map[string]any, error) {
	allowed, reason, _, e := s.notificationEligibility(ctx, id, supplied)
	return map[string]any{"allowed": allowed, "reason": reason}, e
}
func (s *Service) notificationEligibility(ctx context.Context, id string, supplied json.RawMessage) (bool, string, map[string]any, error) {
	var uid, key, reference, email, encrypted, state string
	var expires time.Time
	e := s.DB.QueryRowContext(ctx, `SELECT owner_id,workflow,reference,recipient_email,payload_enc,expires_at,state FROM notification_intents WHERE id=$1`, id).Scan(&uid, &key, &reference, &email, &encrypted, &expires, &state)
	if e != nil {
		return false, "not_found", nil, isMissing(e)
	}
	if !expires.After(s.Now()) || state == "suppressed" || state == "expired" || state == "dead" {
		return false, "expired-or-suppressed", nil, nil
	}
	raw, e := s.Config.Box.Open(encrypted, "notification:"+id)
	if e != nil {
		return false, "payload_unavailable", nil, e
	}
	var payload map[string]any
	if e = json.Unmarshal([]byte(raw), &payload); e != nil {
		return false, "invalid_payload", nil, e
	}
	if len(supplied) > 0 {
		var proposed map[string]any
		if e = json.Unmarshal(supplied, &proposed); e != nil {
			return false, "payload_mismatch", nil, Invalid("invalid policy payload")
		}
		if !security.Equal(security.Digest(jsonText(proposed)), security.Digest(jsonText(payload))) {
			return false, "payload_mismatch", nil, nil
		}
	}
	u, e := s.user(ctx, s.DB, uid, false)
	if e != nil {
		return false, "recipient_missing", nil, e
	}
	contactCode := key == "identity-contact-old-code" || key == "identity-contact-new-code"
	if (!contactCode && u.Email != email) || u.Status == "closed" {
		return false, "recipient_changed", nil, nil
	}
	var suppressed bool
	if e = s.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM suppressions WHERE email_hash=$1)`, security.MAC(s.Config.Pepper, "email:"+email)).Scan(&suppressed); e != nil {
		return false, "suppression_unavailable", nil, e
	}
	if suppressed {
		return false, "mailbox_suppressed", nil, nil
	}
	if contactCode {
		var encrypted, oldHash string
		var valid bool
		e = s.DB.QueryRowContext(ctx, `SELECT new_value_enc,old_value_hash,NOT consumed AND attempts<5 AND expires_at>$3 FROM account_changes WHERE id=$1 AND owner_id=$2`, reference, uid, s.Now()).Scan(&encrypted, &oldHash, &valid)
		if errors.Is(e, sql.ErrNoRows) {
			return false, "contact_change_missing", nil, nil
		}
		if e != nil {
			return false, "contact_change_unavailable", nil, e
		}
		if !valid || !security.Equal(oldHash, security.MAC(s.Config.Pepper, u.Email)) {
			return false, "contact_change_expired", nil, nil
		}
		destination := u.Email
		if key == "identity-contact-new-code" {
			destination, e = s.Config.Box.Open(encrypted, "contact-change:"+reference)
			if e != nil {
				return false, "contact_change_unavailable", nil, e
			}
		}
		if email != destination {
			return false, "contact_destination_mismatch", nil, nil
		}
	} else if NotificationMetadata[key].Code {
		var valid bool
		e = s.DB.QueryRowContext(ctx, `SELECT NOT consumed AND attempts<5 AND expires_at>$2 FROM challenges WHERE id=$1 AND user_id=$3`, reference, s.Now(), uid).Scan(&valid)
		if errors.Is(e, sql.ErrNoRows) {
			return false, "challenge_missing", nil, nil
		}
		if e != nil {
			return false, "challenge_unavailable", nil, e
		}
		if !valid {
			return false, "challenge_expired", nil, nil
		}
	}
	if key == "transfer-submitted" || key == "transfer-pending" || key == "bills-order-received" || key == "bills-order-pending" {
		var financial string
		if e = s.DB.QueryRowContext(ctx, `SELECT status FROM payments WHERE id=$1`, reference).Scan(&financial); e != nil {
			return false, "operation_unavailable", nil, e
		}
		if financial == "succeeded" || financial == "failed" {
			return false, "superseded", nil, nil
		}
	}
	return true, "eligible", payload, nil
}
func (s *Service) WorkNotification(ctx context.Context) (bool, error) {
	if s.Config.NotificationMode != "novu" {
		return false, nil
	}
	var id, key, email, uid string
	e := s.transact(ctx, func(tx *sql.Tx) error {
		// An expired lease is an ambiguous previous submission, not permission to send again.
		if e := exec(tx, ctx, `UPDATE notification_intents SET state='unknown' WHERE state='leased' AND lease_until<$1`, s.Now()); e != nil {
			return e
		}
		e := tx.QueryRowContext(ctx, `UPDATE notification_intents SET state='leased',attempts=attempts+1,lease_until=$1 WHERE id=(SELECT id FROM notification_intents WHERE state='queued' AND available_at<=$2 ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id,workflow,recipient_email,owner_id`, s.Now().Add(90*time.Second), s.Now()).Scan(&id, &key, &email, &uid)
		if errors.Is(e, sql.ErrNoRows) {
			return nil
		}
		return e
	})
	if e == nil && id == "" {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	workflow := "qpf-email-" + key + "-v1"
	if !s.Config.NovuAllowlist[workflow] {
		_, e = s.DB.ExecContext(ctx, `UPDATE notification_intents SET state='queued',available_at=$2,lease_until=NULL WHERE id=$1`, id, s.Now().Add(time.Hour))
		return true, e
	}
	allowed, _, payload, e := s.notificationEligibility(ctx, id, nil)
	if e != nil {
		_, update := s.DB.ExecContext(ctx, `UPDATE notification_intents SET state='queued',available_at=$2,lease_until=NULL WHERE id=$1`, id, s.Now().Add(time.Minute))
		if update != nil {
			return true, update
		}
		return true, e
	}
	if !allowed {
		_, e = s.DB.ExecContext(ctx, `UPDATE notification_intents SET state='suppressed',payload_enc=CASE WHEN secret THEN '' ELSE payload_enc END,lease_until=NULL WHERE id=$1`, id)
		return true, e
	}
	body := map[string]any{"name": workflow, "transactionId": id, "to": map[string]any{"subscriberId": uid, "email": email}, "payload": payload}
	request, e := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.Config.NovuURL, "/")+"/v1/events/trigger", bytes.NewBufferString(jsonText(body)))
	if e != nil {
		return true, e
	}
	request.Header.Set("Authorization", "ApiKey "+s.Config.NovuKey)
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirects disabled") }}
	response, e := client.Do(request)
	state := "unknown"
	if e == nil {
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			state = "submitted"
		} else if response.StatusCode >= 400 && response.StatusCode < 500 && response.StatusCode != 408 && response.StatusCode != 429 {
			state = "dead"
		}
	}
	_, update := s.DB.ExecContext(ctx, `UPDATE notification_intents SET state=$2,lease_until=NULL WHERE id=$1 AND state='leased'`, id, state)
	if update != nil {
		return true, update
	}
	return true, e
}

// LocalChallenge is intentionally reachable only through the local-only command, never HTTP.
func (s *Service) LocalChallenge(ctx context.Context, email string) (map[string]any, error) {
	if s.Config.Environment != "local" || s.Config.NotificationMode != "local" {
		return nil, denied()
	}
	var id, encrypted string
	e := s.DB.QueryRowContext(ctx, `SELECT n.id,n.payload_enc FROM notification_intents n WHERE n.recipient_email=$1 AND n.secret AND n.expires_at>$2 ORDER BY n.created_at DESC LIMIT 1`, strings.ToLower(email), s.Now()).Scan(&id, &encrypted)
	if e != nil {
		return nil, isMissing(e)
	}
	raw, e := s.Config.Box.Open(encrypted, "notification:"+id)
	if e != nil {
		return nil, e
	}
	var out map[string]any
	e = json.Unmarshal([]byte(raw), &out)
	return out, e
}
