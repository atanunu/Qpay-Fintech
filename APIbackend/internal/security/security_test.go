package security

import (
	"bytes"
	"encoding/base32"
	"strings"
	"testing"
	"time"
)

func TestPasswordAndPIN(t *testing.T) {
	pepper := bytes.Repeat([]byte{3}, 32)
	hash, e := Password("synthetic-password-123", pepper)
	if e != nil {
		t.Fatal(e)
	}
	if !Verify("synthetic-password-123", hash, pepper) {
		t.Fatal("valid password rejected")
	}
	for name, trial := range map[string]struct {
		value, hash string
		pepper      []byte
	}{"wrong password": {"different-password", hash, pepper}, "wrong pepper": {"synthetic-password-123", hash, bytes.Repeat([]byte{4}, 32)}, "hostile cost": {"synthetic-password-123", strings.Replace(hash, "m=32768", "m=999999999", 1), pepper}, "truncated": {"synthetic-password-123", "$argon2id$", pepper}} {
		t.Run(name, func(t *testing.T) {
			if Verify(trial.value, trial.hash, trial.pepper) {
				t.Fatal("invalid password accepted")
			}
		})
	}
	if _, e = Password("short", pepper); e == nil {
		t.Fatal("short password accepted")
	}
	if _, e = PIN("12345x", pepper); e == nil {
		t.Fatal("invalid PIN accepted")
	}
	pin, e := PIN("123456", pepper)
	if e != nil || !Verify("123456", pin, pepper) {
		t.Fatal("PIN hash failure")
	}
}
func TestAuthenticatedEncryptionAndRotation(t *testing.T) {
	first := bytes.Repeat([]byte{1}, 32)
	second := bytes.Repeat([]byte{2}, 32)
	old := Box{Active: "v1", Keys: map[string][]byte{"v1": first}}
	sealed, e := old.Seal("synthetic confidential data", "owner:one")
	if e != nil {
		t.Fatal(e)
	}
	rotated := Box{Active: "v2", Keys: map[string][]byte{"v1": first, "v2": second}}
	plain, e := rotated.Open(sealed, "owner:one")
	if e != nil || plain != "synthetic confidential data" {
		t.Fatal("rotation could not recover old data")
	}
	if _, e = rotated.Open(sealed, "owner:two"); e == nil {
		t.Fatal("cross-owner decryption succeeded")
	}
	parts := strings.Split(sealed, ".")
	if parts[1][0] == 'A' {
		parts[1] = "B" + parts[1][1:]
	} else {
		parts[1] = "A" + parts[1][1:]
	}
	if _, e = rotated.Open(strings.Join(parts, "."), "owner:one"); e == nil {
		t.Fatal("tampered ciphertext accepted")
	}
	if _, e = (Box{Active: "missing", Keys: map[string][]byte{}}).Seal("value", "owner"); e == nil {
		t.Fatal("missing key accepted")
	}
}
func TestRFC6238SHA1VectorsAndReplay(t *testing.T) {
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	for _, v := range []struct {
		at   int64
		code string
	}{{59, "287082"}, {1111111109, "081804"}, {1111111111, "050471"}, {1234567890, "005924"}, {2000000000, "279037"}, {20000000000, "353130"}} {
		t.Run(v.code, func(t *testing.T) {
			got, e := TOTP(secret, v.at/30)
			if e != nil || got != v.code {
				t.Fatalf("got %q: %v", got, e)
			}
			counter, ok := VerifyTOTP(secret, v.code, time.Unix(v.at, 0), -1)
			if !ok {
				t.Fatal("valid TOTP rejected")
			}
			if _, ok = VerifyTOTP(secret, v.code, time.Unix(v.at, 0), counter); ok {
				t.Fatal("TOTP replay accepted")
			}
		})
	}
}
func TestWebhookSignatureWindow(t *testing.T) {
	key := bytes.Repeat([]byte{8}, 32)
	now := time.Unix(2000000000, 0)
	body := `{"id":"synthetic"}`
	timestamp := "2000000000"
	signature := MAC(key, timestamp+"."+body)
	if !VerifySignature(key, timestamp, body, signature, now) {
		t.Fatal("valid signature rejected")
	}
	if VerifySignature(key, timestamp, body+" ", signature, now) {
		t.Fatal("altered body accepted")
	}
	if VerifySignature(key, timestamp, body, signature, now.Add(6*time.Minute)) {
		t.Fatal("expired signature accepted")
	}
}
