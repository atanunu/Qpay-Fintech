// Package security contains bounded authentication and authenticated-encryption primitives.
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" // RFC 6238 interoperability; not used for password storage.
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

var ErrInvalid = errors.New("invalid authentication material")

const memory = uint32(32 * 1024)
const iterations = uint32(3)

func Random(prefix string, n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("operating system random source unavailable")
	}
	return prefix + base64.RawURLEncoding.EncodeToString(b)
}
func Digest(value string) string { h := sha256.Sum256([]byte(value)); return hex.EncodeToString(h[:]) }
func MAC(key []byte, value string) string {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}
func Equal(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }
func Password(password string, pepper []byte) (string, error) {
	if len(password) < 12 || len(password) > 256 {
		return "", errors.New("password must contain 12 to 256 bytes")
	}
	return hashSecret(password, pepper)
}
func PIN(pin string, pepper []byte) (string, error) {
	if len(pin) != 6 || strings.Trim(pin, "0123456789") != "" {
		return "", errors.New("transaction PIN must contain six digits")
	}
	return hashSecret(pin, pepper)
}
func hashSecret(value string, pepper []byte) (string, error) {
	if len(pepper) != 32 {
		return "", ErrInvalid
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := argon2.IDKey([]byte(MAC(pepper, "password:"+value)), salt, iterations, memory, 1, 32)
	return fmt.Sprintf("$argon2id$v=19$m=32768,t=3,p=1$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(sum)), nil
}
func Verify(value, encoded string, pepper []byte) bool {
	if len(value) > 256 || len(pepper) != 32 {
		return false
	}
	fields := strings.Split(encoded, "$")
	if len(fields) != 6 || fields[1] != "argon2id" || fields[2] != "v=19" || fields[3] != "m=32768,t=3,p=1" {
		return false
	}
	salt, e1 := base64.RawStdEncoding.DecodeString(fields[4])
	sum, e2 := base64.RawStdEncoding.DecodeString(fields[5])
	if e1 != nil || e2 != nil || len(salt) != 16 || len(sum) != 32 {
		return false
	}
	actual := argon2.IDKey([]byte(MAC(pepper, "password:"+value)), salt, iterations, memory, 1, 32)
	return subtle.ConstantTimeCompare(actual, sum) == 1
}
func Code() string {
	n, e := rand.Int(rand.Reader, big.NewInt(1000000))
	if e != nil {
		panic("random source unavailable")
	}
	return fmt.Sprintf("%06d", n.Int64())
}

// Box separates key identity from ciphertext and binds values to their purpose and owner.
type Box struct {
	Active string
	Keys   map[string][]byte
}

func (b Box) Seal(plain, purpose string) (string, error) {
	key, ok := b.Keys[b.Active]
	if !ok || len(key) != 32 || strings.Contains(b.Active, ".") {
		return "", ErrInvalid
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	a, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	out := a.Seal(nonce, nonce, []byte(plain), []byte(purpose))
	return b.Active + "." + base64.RawURLEncoding.EncodeToString(out), nil
}
func (b Box) Open(sealed, purpose string) (string, error) {
	parts := strings.SplitN(sealed, ".", 2)
	if len(parts) != 2 {
		return "", ErrInvalid
	}
	key, ok := b.Keys[parts[0]]
	if !ok || len(key) != 32 {
		return "", ErrInvalid
	}
	raw, e := base64.RawURLEncoding.DecodeString(parts[1])
	if e != nil {
		return "", ErrInvalid
	}
	block, e := aes.NewCipher(key)
	if e != nil {
		return "", e
	}
	a, e := cipher.NewGCM(block)
	if e != nil {
		return "", e
	}
	if len(raw) < a.NonceSize()+a.Overhead() {
		return "", ErrInvalid
	}
	plain, e := a.Open(nil, raw[:a.NonceSize()], raw[a.NonceSize():], []byte(purpose))
	if e != nil {
		return "", ErrInvalid
	}
	return string(plain), nil
}
func NewTOTPSecret() string {
	b := make([]byte, 20)
	if _, e := rand.Read(b); e != nil {
		panic("random source unavailable")
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
}
func TOTP(secret string, counter int64) (string, error) {
	if counter < 0 {
		return "", ErrInvalid
	}
	key, e := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if e != nil || len(key) < 20 {
		return "", ErrInvalid
	}
	var raw [8]byte
	binary.BigEndian.PutUint64(raw[:], uint64(counter))
	h := hmac.New(sha1.New, key)
	_, _ = h.Write(raw[:])
	sum := h.Sum(nil)
	o := sum[len(sum)-1] & 15
	n := (uint32(sum[o])&127)<<24 | uint32(sum[o+1])<<16 | uint32(sum[o+2])<<8 | uint32(sum[o+3])
	return fmt.Sprintf("%06d", n%1000000), nil
}
func VerifyTOTP(secret, code string, now time.Time, last int64) (int64, bool) {
	if len(code) != 6 || strings.Trim(code, "0123456789") != "" {
		return 0, false
	}
	for _, delta := range []int64{0, -1, 1} {
		c := now.Unix()/30 + delta
		if c <= last {
			continue
		}
		expected, e := TOTP(secret, c)
		if e == nil && Equal(expected, code) {
			return c, true
		}
	}
	return 0, false
}
func VerifySignature(key []byte, timestamp, body, signature string, now time.Time) bool {
	n, e := strconv.ParseInt(timestamp, 10, 64)
	if e != nil || n < now.Add(-5*time.Minute).Unix() || n > now.Add(30*time.Second).Unix() {
		return false
	}
	expected := MAC(key, timestamp+"."+body)
	return Equal(expected, strings.TrimPrefix(signature, "sha256="))
}
