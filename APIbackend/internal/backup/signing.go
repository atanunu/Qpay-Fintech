package backup

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

func Sign(v any, key ed25519.PrivateKey) (Signed, error) {
	if len(key) != ed25519.PrivateKeySize {
		return Signed{}, errors.New("backup signing key is not configured")
	}
	raw, e := json.Marshal(v)
	if e != nil {
		return Signed{}, e
	}
	return Signed{raw, base64.StdEncoding.EncodeToString(ed25519.Sign(key, raw))}, nil
}
func Verify(in Signed, key ed25519.PublicKey, out any) error {
	if len(key) != ed25519.PublicKeySize || len(in.Payload) > 8<<20 {
		return errors.New("invalid backup signing envelope")
	}
	sig, e := base64.StdEncoding.DecodeString(in.Signature)
	if e != nil || !ed25519.Verify(key, in.Payload, sig) {
		return errors.New("invalid backup signature")
	}
	return Decode(in.Payload, out)
}
func VerifyPolicy(in Signed, key ed25519.PublicKey, agent, environment string, now time.Time, minSerial int64) (Policy, error) {
	var p Policy
	if e := Verify(in, key, &p); e != nil {
		return p, e
	}
	if p.Version != SchemaVersion || p.AgentID != agent || p.Environment != environment || p.Serial < minSerial || p.IssuedAt.After(now.Add(time.Minute)) || !p.ExpiresAt.After(now) || p.ExpiresAt.Sub(p.IssuedAt) > 24*time.Hour || p.ExpiresAt.Before(p.IssuedAt) {
		return p, errors.New("policy expired, rolled back or belongs to another agent")
	}
	return p, nil
}

// HTTP authentication signs method/path/time/nonce and a body digest. Agent
// identities and public keys are provisioned outside the business database.
type Authentication struct {
	AgentID    string    `json:"agent_id"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Timestamp  time.Time `json:"timestamp"`
	Nonce      string    `json:"nonce"`
	BodyDigest string    `json:"body_digest"`
}

func EncodeAuth(a Authentication, key ed25519.PrivateKey) (string, error) {
	s, e := Sign(a, key)
	if e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(Canonical(s)), nil
}
func DecodeAuth(header string, keys map[string]ed25519.PublicKey, method, path string, body []byte, now time.Time) (Authentication, error) {
	var a Authentication
	if len(header) > 4096 {
		return a, errors.New("invalid agent authorization")
	}
	b, e := base64.RawURLEncoding.DecodeString(header)
	if e != nil {
		return a, e
	}
	var s Signed
	if e = Decode(b, &s); e != nil {
		return a, e
	}
	if e = Decode(s.Payload, &a); e != nil {
		return a, e
	}
	if e = Verify(s, keys[a.AgentID], &a); e != nil {
		return a, e
	}
	if a.Method != method || a.Path != path || a.BodyDigest != Digest(body) || !ID(a.Nonce) || a.Timestamp.Before(now.Add(-5*time.Minute)) || a.Timestamp.After(now.Add(time.Minute)) {
		return a, errors.New("invalid, stale or mismatched agent authorization")
	}
	return a, nil
}
