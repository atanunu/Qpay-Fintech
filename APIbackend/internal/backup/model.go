// Package backup contains the backup control contracts and isolated execution engine.
// It has no dependency on the payment service and cannot submit financial work.
package backup

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
)

const SchemaVersion = 1

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{1,99}$`)
var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func ID(s string) bool       { return identifier.MatchString(s) }
func Digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func Text(s string, n int) bool {
	return len(strings.TrimSpace(s)) > 0 && len(s) <= n && !strings.ContainsFunc(s, unicode.IsControl)
}
func Decode(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return errors.New("invalid or unsupported configuration field")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("one JSON object is required")
	}
	return nil
}
func Canonical(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

type Ref struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
}

func (r Ref) Valid() bool { return ID(r.ID) && r.Version > 0 }
func (r Ref) Key() string { return fmt.Sprintf("%s@%d", r.ID, r.Version) }

type Resource struct {
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	Name        string          `json:"name"`
	Environment string          `json:"environment"`
	AgentID     string          `json:"agent_id"`
	Version     int64           `json:"version"`
	State       string          `json:"state"`
	Disabled    bool            `json:"disabled"`
	Digest      string          `json:"digest"`
	Spec        json.RawMessage `json:"spec"`
	CreatedBy   string          `json:"created_by"`
	ApprovedBy  string          `json:"approved_by"`
	CreatedAt   time.Time       `json:"created_at"`
	DecidedAt   *time.Time      `json:"decided_at,omitempty"`
}

func (r Resource) Ref() Ref { return Ref{r.ID, r.Version} }
func Kind(k string) bool {
	return slices.Contains([]string{"sources", "destinations", "plans", "schedules", "retention"}, k)
}

type SourceSpec struct {
	Profile        string `json:"profile"`
	Type           string `json:"type"`
	Classification string `json:"classification"`
	RPOSeconds     int    `json:"rpo_seconds"`
	Description    string `json:"description"`
}
type DestinationSpec struct {
	Profile       string `json:"profile"`
	Provider      string `json:"provider"`
	FailureDomain string `json:"failure_domain"`
	OffHost       bool   `json:"off_host"`
	Region        string `json:"region"`
	Description   string `json:"description"`
}
type CopyBinding struct {
	Destination Ref  `json:"destination"`
	Required    bool `json:"required"`
}
type PlanSpec struct {
	Sources           []Ref         `json:"sources"`
	Destinations      []CopyBinding `json:"destinations"`
	Retention         Ref           `json:"retention"`
	Verify            string        `json:"verify"`
	MinFailureDomains int           `json:"min_failure_domains"`
	TimeoutSeconds    int           `json:"timeout_seconds"`
}
type RetentionSpec struct {
	KeepLast    int `json:"keep_last"`
	KeepHourly  int `json:"keep_hourly"`
	KeepDaily   int `json:"keep_daily"`
	KeepWeekly  int `json:"keep_weekly"`
	KeepMonthly int `json:"keep_monthly"`
	MinimumDays int `json:"minimum_days"`
}
type ScheduleSpec struct {
	Plan            Ref        `json:"plan"`
	Operation       string     `json:"operation"`
	Timezone        string     `json:"timezone"`
	Frequency       string     `json:"frequency"`
	At              string     `json:"at"`
	Weekdays        []int      `json:"weekdays"`
	MonthDay        int        `json:"month_day"`
	IntervalMinutes int        `json:"interval_minutes"`
	StartsAt        time.Time  `json:"starts_at"`
	EndsAt          *time.Time `json:"ends_at,omitempty"`
	Missed          string     `json:"missed"`
}

type Profile struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	Type          string   `json:"type"`
	Label         string   `json:"label"`
	Location      string   `json:"location"`
	Account       string   `json:"account"`
	Region        string   `json:"region"`
	Capabilities  []string `json:"capabilities"`
	Provider      string   `json:"provider"`
	FailureDomain string   `json:"failure_domain"`
	OffHost       bool     `json:"off_host"`
}
type AgentStatus struct {
	RunID           string            `json:"run_id,omitempty"`
	Stage           string            `json:"stage,omitempty"`
	ID              string            `json:"id"`
	Environment     string            `json:"environment"`
	InstanceID      string            `json:"instance_id"`
	ObservedAt      time.Time         `json:"observed_at"`
	Versions        map[string]string `json:"versions"`
	Profiles        []Profile         `json:"profiles"`
	PolicyExpiresAt time.Time         `json:"policy_expires_at"`
	PendingResults  int               `json:"pending_results"`
	FreeBytes       string            `json:"free_bytes"`
}
type Provider struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Transport     string `json:"transport"`
	Help          string `json:"help"`
	Qualification string `json:"qualification"`
}

var Providers = []Provider{
	{"local", "Local / mounted recovery repository", "restic", "Choose a dedicated approved repository. A same-host copy is not off-host protection.", "local_integration"},
	{"s3", "S3-compatible object storage", "restic", "Use a scoped repository profile. Check account, bucket, region, versioning and retention separately.", "adapter_available"},
	{"dropbox", "Dropbox", "rclone", "Use an organization-owned scoped app-folder connection and offline access. Reconnect the named profile on the agent; never paste tokens in this form.", "adapter_available"},
	{"onedrive", "Microsoft OneDrive", "rclone", "Use a reviewed tenant, drive and scope. Reauthorize the named connection profile when access expires.", "adapter_available"},
	{"sharepoint", "SharePoint document library", "rclone", "Approve the exact tenant/library and scopes; no automatic tenant-wide permission grant.", "adapter_available"},
	{"gdrive", "Google Drive / Shared Drive", "rclone", "Use an approved drive or service account connection. Confirm ownership survives staff departure.", "adapter_available"},
	{"azure", "Azure Blob Storage", "restic", "Use a dedicated container with separately managed write and maintenance credentials.", "adapter_available"},
	{"gcs", "Google Cloud Storage", "restic", "Use a scoped service identity and an approved bucket/region.", "adapter_available"},
	{"sftp", "SFTP backup server", "restic", "Pin the host key and use an isolated account. Unknown host keys must fail closed.", "adapter_available"},
	{"webdav", "Nextcloud / ownCloud / HTTPS WebDAV", "rclone", "Use HTTPS and a dedicated account/path. A synchronized folder is not immutable storage.", "adapter_available"},
	{"box", "Box", "rclone", "Use an approved business connection profile and verify upload limits and retention.", "adapter_available"},
	{"b2", "Backblaze B2", "restic", "Use a dedicated bucket and application key. Qualify deletion protection independently.", "adapter_available"},
	{"pgbackrest", "PostgreSQL physical repository", "pgbackrest", "Each configured repository is backed up separately. File-cloud mounts are not supported WAL repositories.", "adapter_available"},
}

// ValidateSpec rejects arbitrary commands, paths, URLs and secrets: these live only in
// the provisioned agent profile. The web app selects non-secret profile identities.
func ValidateSpec(kind string, raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > 32768 {
		return nil, errors.New("configuration must be at most 32 KiB")
	}
	var value any
	switch kind {
	case "sources":
		var v SourceSpec
		if e := Decode(raw, &v); e != nil {
			return nil, e
		}
		if !ID(v.Profile) || !slices.Contains([]string{"files", "postgres_logical", "qpf_postgres", "postgres_physical", "mongodb", "redis"}, v.Type) || !slices.Contains([]string{"internal", "confidential", "restricted"}, v.Classification) || v.RPOSeconds < 60 || v.RPOSeconds > 2592000 || len(v.Description) > 500 {
			return nil, errors.New("choose a supported source profile, classification and recovery objective")
		}
		value = v
	case "destinations":
		var v DestinationSpec
		if e := Decode(raw, &v); e != nil {
			return nil, e
		}
		known := false
		for _, p := range Providers {
			known = known || p.ID == v.Provider
		}
		if !ID(v.Profile) || !known || !Text(v.FailureDomain, 100) || !Text(v.Region, 100) || len(v.Description) > 500 {
			return nil, errors.New("provider, profile, failure domain and region are required")
		}
		value = v
	case "plans":
		var v PlanSpec
		if e := Decode(raw, &v); e != nil {
			return nil, e
		}
		if len(v.Sources) < 1 || len(v.Sources) > 20 || len(v.Destinations) < 1 || len(v.Destinations) > 10 || !v.Retention.Valid() || v.MinFailureDomains < 1 || v.MinFailureDomains > 10 || v.TimeoutSeconds < 60 || v.TimeoutSeconds > 86400 || !slices.Contains([]string{"metadata", "read_data"}, v.Verify) {
			return nil, errors.New("select sources, destinations, retention and bounded verification settings")
		}
		seen := map[string]bool{}
		for _, r := range v.Sources {
			if !r.Valid() || seen[r.ID] {
				return nil, errors.New("source bindings must be unique exact revisions")
			}
			seen[r.ID] = true
		}
		seen = map[string]bool{}
		required := 0
		for _, b := range v.Destinations {
			if !b.Destination.Valid() || seen[b.Destination.ID] {
				return nil, errors.New("destination bindings must be unique exact revisions")
			}
			seen[b.Destination.ID] = true
			if b.Required {
				required++
			}
		}
		if required < v.MinFailureDomains {
			return nil, errors.New("too few required destinations for the independent-copy policy")
		}
		value = v
	case "schedules":
		var v ScheduleSpec
		if e := Decode(raw, &v); e != nil {
			return nil, e
		}
		if e := ValidateSchedule(v); e != nil {
			return nil, e
		}
		value = v
	case "retention":
		var v RetentionSpec
		if e := Decode(raw, &v); e != nil {
			return nil, e
		}
		if v.KeepLast < 2 || v.KeepLast > 10000 || v.MinimumDays < 1 || v.MinimumDays > 3650 {
			return nil, errors.New("retain at least two snapshots and a positive minimum recovery window")
		}
		for _, n := range []int{v.KeepHourly, v.KeepDaily, v.KeepWeekly, v.KeepMonthly} {
			if n < 0 || n > 10000 {
				return nil, errors.New("retention counts must be 0 to 10000")
			}
		}
		value = v
	default:
		return nil, errors.New("unknown backup resource")
	}
	return Canonical(value), nil
}
func Parse[T any](r Resource) T { var v T; _ = json.Unmarshal(r.Spec, &v); return v }

type Command struct {
	ID            string     `json:"id"`
	Operation     string     `json:"operation"`
	Resource      Ref        `json:"resource"`
	PointID       string     `json:"point_id,omitempty"`
	CopyID        string     `json:"copy_id,omitempty"`
	Target        string     `json:"target,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	ExpiresAt     time.Time  `json:"expires_at"`
	Approved      bool       `json:"approved"`
	RecoveryTime  *time.Time `json:"recovery_time,omitempty"`
	PreviewDigest string     `json:"preview_digest,omitempty"`
}
type Policy struct {
	Version     int        `json:"version"`
	Serial      int64      `json:"serial"`
	AgentID     string     `json:"agent_id"`
	Environment string     `json:"environment"`
	IssuedAt    time.Time  `json:"issued_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	Resources   []Resource `json:"resources"`
	Commands    []Command  `json:"commands"`
	Holds       []string   `json:"holds,omitempty"`
}
type Signed struct {
	Payload   json.RawMessage `json:"payload"`
	Signature string          `json:"signature"`
}
type Copy struct {
	Dependencies    []string  `json:"dependencies,omitempty"`
	WALStart        string    `json:"wal_start,omitempty"`
	WALStop         string    `json:"wal_stop,omitempty"`
	ID              string    `json:"id"`
	Source          Ref       `json:"source"`
	Destination     Ref       `json:"destination"`
	Engine          string    `json:"engine"`
	Snapshot        string    `json:"snapshot"`
	State           string    `json:"state"`
	Bytes           string    `json:"bytes"`
	ErrorCode       string    `json:"error_code,omitempty"`
	Verified        string    `json:"verified"`
	CaptureStarted  time.Time `json:"capture_started"`
	CaptureFinished time.Time `json:"capture_finished"`
}
type File struct {
	Path          string `json:"path"`
	Size          string `json:"size"`
	SHA256        string `json:"sha256,omitempty"`
	SourceID      string `json:"source_id"`
	ObjectVersion string `json:"object_version,omitempty"`
}
type Result struct {
	Version            int               `json:"version"`
	AgentID            string            `json:"agent_id"`
	RunID              string            `json:"run_id"`
	PolicyDigest       string            `json:"policy_digest"`
	CommandID          string            `json:"command_id,omitempty"`
	Schedule           Ref               `json:"schedule"`
	Plan               Ref               `json:"plan"`
	Occurrence         time.Time         `json:"occurrence"`
	Operation          string            `json:"operation"`
	State              string            `json:"state"`
	StartedAt          time.Time         `json:"started_at"`
	FinishedAt         time.Time         `json:"finished_at"`
	Copies             []Copy            `json:"copies"`
	Files              []File            `json:"files"`
	FilesTruncated     bool              `json:"files_truncated"`
	ErrorCode          string            `json:"error_code,omitempty"`
	PointID            string            `json:"point_id,omitempty"`
	MaterializedTarget string            `json:"materialized_target,omitempty"`
	Retention          *RetentionPreview `json:"retention,omitempty"`
	ExpiredCopyID      string            `json:"expired_copy_id,omitempty"`
}
