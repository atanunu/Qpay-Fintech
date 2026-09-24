package backup

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Health is deliberately content-free so an independent monitor needs only the
// agent public key, not a database credential or backup decryption key.
type Health struct {
	Version               int       `json:"version"`
	AgentID               string    `json:"agent_id"`
	ObservedAt            time.Time `json:"observed_at"`
	PolicyExpiresAt       time.Time `json:"policy_expires_at"`
	LastSuccessfulCapture time.Time `json:"last_successful_capture"`
	PendingResults        int       `json:"pending_results"`
	RunID                 string    `json:"run_id,omitempty"`
	Stage                 string    `json:"stage"`
}

func WriteHealth(dir string, h Health, key ed25519.PrivateKey) error {
	signed, err := Sign(h, key)
	if err != nil {
		return err
	}
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() || fi.Mode().Perm()&0077 != 0 {
		return errors.New("health directory must be protected")
	}
	file, err := os.CreateTemp(dir, "health-*.tmp")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.Write(Canonical(signed)); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(dir, "health.json"))
}
func CheckHealth(file string, public ed25519.PublicKey, now time.Time, maxAge, maxCaptureAge time.Duration) (Health, error) {
	var h Health
	fi, err := os.Lstat(file)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > 16384 {
		return h, errors.New("backup health evidence is missing or invalid")
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return h, err
	}
	var signed Signed
	if err = json.Unmarshal(raw, &signed); err != nil {
		return h, err
	}
	if err = Verify(signed, public, &h); err != nil {
		return h, err
	}
	if h.Version != 1 || !ID(h.AgentID) || maxAge <= 0 || maxCaptureAge <= 0 {
		return h, errors.New("invalid monitor configuration")
	}
	if h.ObservedAt.After(now.Add(time.Minute)) || now.Sub(h.ObservedAt) > maxAge {
		return h, errors.New("backup agent heartbeat is stale")
	}
	if !h.PolicyExpiresAt.After(now) {
		return h, errors.New("backup execution policy has expired")
	}
	if h.LastSuccessfulCapture.IsZero() || now.Sub(h.LastSuccessfulCapture) > maxCaptureAge {
		return h, errors.New("required backup capture is missing or stale")
	}
	if h.PendingResults >= 512 {
		return h, errors.New("backup result reporting is blocked")
	}
	return h, nil
}
