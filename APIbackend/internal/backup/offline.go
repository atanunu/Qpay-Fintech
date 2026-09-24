package backup

import (
	"context"
	"crypto/ed25519"
	"errors"
	"os"
	"time"
)

// OfflineBundle is confidential metadata: carry it on separately encrypted media.
// It contains no provider passwords or decryption keys. Two external custodians
// authorize the same exact bundle and isolated target when the API is unavailable.
type OfflineRequest struct {
	Version      int       `json:"version"`
	ID           string    `json:"id"`
	AgentID      string    `json:"agent_id"`
	Environment  string    `json:"environment"`
	PolicyDigest string    `json:"policy_digest"`
	PointDigest  string    `json:"point_digest"`
	CopyID       string    `json:"copy_id"`
	Target       string    `json:"target"`
	Operation    string    `json:"operation"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	NoDispatch   bool      `json:"no_dispatch_acknowledged"`
}
type OfflineApproval struct {
	Custodian string `json:"custodian"`
	Signed    Signed `json:"signed"`
}
type OfflineBundle struct {
	Request   OfflineRequest    `json:"request"`
	Policy    Signed            `json:"policy"`
	Point     Signed            `json:"point"`
	Approvals []OfflineApproval `json:"approvals"`
}

func NewOfflineBundle(entry JournalRun, id, copyID, target, operation string, now time.Time) (OfflineBundle, error) {
	if !entry.Complete || !ID(id) || !ID(copyID) || !ID(target) || (operation != "restore" && operation != "export") {
		return OfflineBundle{}, errors.New("a finalized recovery point and explicit isolated target are required")
	}
	if _, err := selectedCopy(entry.Result, copyID); err != nil {
		return OfflineBundle{}, err
	}
	var p Policy
	if err := Decode(entry.Policy.Payload, &p); err != nil {
		return OfflineBundle{}, err
	}
	return OfflineBundle{Request: OfflineRequest{Version: 1, ID: id, AgentID: p.AgentID, Environment: p.Environment, PolicyDigest: Digest(Canonical(entry.Policy)), PointDigest: Digest(Canonical(entry.Signed)), CopyID: copyID, Target: target, Operation: operation, CreatedAt: now, ExpiresAt: now.Add(time.Hour), NoDispatch: true}, Policy: entry.Policy, Point: entry.Signed, Approvals: []OfflineApproval{}}, nil
}
func ApproveOffline(b OfflineBundle, id string, key ed25519.PrivateKey) (OfflineBundle, error) {
	if !ID(id) || len(key) != ed25519.PrivateKeySize || len(b.Approvals) >= 4 {
		return b, errors.New("valid independent custodian and at most four approvals required")
	}
	for _, a := range b.Approvals {
		if a.Custodian == id {
			return b, errors.New("custodian already approved")
		}
	}
	if b.Request.PolicyDigest != Digest(Canonical(b.Policy)) || b.Request.PointDigest != Digest(Canonical(b.Point)) {
		return b, errors.New("offline evidence was changed")
	}
	signed, err := Sign(b.Request, key)
	if err != nil {
		return b, err
	}
	b.Approvals = append(b.Approvals, OfflineApproval{id, signed})
	return b, nil
}
func (e *Engine) OfflineRecover(ctx context.Context, b OfflineBundle, policyKey, pointKey ed25519.PublicKey, custodians map[string]ed25519.PublicKey) (string, error) {
	r := b.Request
	now := e.Now()
	if r.Version != 1 || !ID(r.ID) || r.AgentID != e.Config.ID || r.Environment != e.Config.Environment || !r.NoDispatch || !ID(r.CopyID) || !ID(r.Target) || (r.Operation != "restore" && r.Operation != "export") || r.CreatedAt.After(now.Add(time.Minute)) || !r.ExpiresAt.After(now) || r.ExpiresAt.Sub(r.CreatedAt) > time.Hour {
		return "", errors.New("invalid, expired or mismatched offline recovery authority")
	}
	if r.PolicyDigest != Digest(Canonical(b.Policy)) || r.PointDigest != Digest(Canonical(b.Point)) {
		return "", errors.New("offline evidence digest mismatch")
	}
	approved := map[string]bool{}
	keys := map[string]bool{}
	for _, approval := range b.Approvals {
		key, ok := custodians[approval.Custodian]
		if !ok || approved[approval.Custodian] || keys[string(key)] {
			return "", errors.New("offline approvals must use distinct enrolled custodians and keys")
		}
		var signedRequest OfflineRequest
		if err := Verify(approval.Signed, key, &signedRequest); err != nil {
			return "", err
		}
		if Digest(Canonical(signedRequest)) != Digest(Canonical(r)) {
			return "", errors.New("approval belongs to different offline request")
		}
		approved[approval.Custodian] = true
		keys[string(key)] = true
	}
	if len(approved) < 2 {
		return "", errors.New("two independent recovery custodian signatures are required")
	}
	var p Policy
	var point Result
	// Historical capture signatures remain useful after their execution lease ends.
	// They never authorize new capture or live financial execution.
	if err := Verify(b.Policy, policyKey, &p); err != nil {
		return "", err
	}
	if err := Verify(b.Point, pointKey, &point); err != nil {
		return "", err
	}
	if p.AgentID != r.AgentID || p.Environment != r.Environment || point.AgentID != r.AgentID || point.PolicyDigest != Digest(b.Policy.Payload) {
		return "", errors.New("historical capture identity mismatch")
	}
	if err := ValidateResult(point); err != nil {
		return "", err
	}
	if err := AuthorizeResult(p, point); err != nil {
		return "", err
	}
	if point.Operation != "backup" && point.Operation != "full" && point.Operation != "diff" && point.Operation != "incr" {
		return "", errors.New("select an original captured recovery point")
	}
	cmd := Command{ID: r.ID, Operation: r.Operation, Resource: point.Plan, PointID: point.RunID, CopyID: r.CopyID, Target: r.Target, Approved: true, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt}
	return e.materialize(ctx, p, cmd, point)
}
func ReadOfflineBundle(file string) (OfflineBundle, error) {
	var b OfflineBundle
	fi, err := os.Lstat(file)
	if err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0077 != 0 || fi.Size() > 16<<20 {
		return b, errors.New("offline bundle requires a bounded owner-only regular file")
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return b, err
	}
	err = Decode(raw, &b)
	return b, err
}
func WriteNewProtected(file string, v any) error {
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.Write(Canonical(v)); err != nil {
		return err
	}
	return f.Sync()
}
