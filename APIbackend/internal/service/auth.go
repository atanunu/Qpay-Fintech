package service

import (
	"context"
	"database/sql"
	"errors"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
)

func normalEmail(value string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(value))
	a, e := mail.ParseAddress(v)
	_, domain, hasAt := strings.Cut(v, "@")
	if e != nil || a.Address != v || len(v) > 254 || strings.ContainsAny(v, "\r\n") || (!hasAt || !strings.Contains(domain, ".")) {
		return "", Invalid("valid email address required")
	}
	return v, nil
}
func (s *Service) Rate(ctx context.Context, key string, max int, period time.Duration) error {
	window := time.Unix(s.Now().Unix()/int64(period.Seconds())*int64(period.Seconds()), 0).UTC()
	var hits int
	e := s.DB.QueryRowContext(ctx, `INSERT INTO rate_limits(bucket,window_at,hits) VALUES($1,$2,1) ON CONFLICT(bucket) DO UPDATE SET window_at=excluded.window_at,hits=CASE WHEN rate_limits.window_at=excluded.window_at THEN rate_limits.hits+1 ELSE 1 END RETURNING hits`, security.MAC(s.Config.Pepper, "rate:"+key), window).Scan(&hits)
	if e != nil {
		return e
	}
	if hits > max {
		return &Fault{429, "rate_limited", "request limit reached; try again later"}
	}
	return nil
}
func (s *Service) Register(ctx context.Context, email, name, password string) error {
	email, e := normalEmail(email)
	if e != nil {
		return e
	}
	if !safeText(name, 80) {
		return Invalid("name must contain 1 to 80 bytes")
	}
	if e = s.Rate(ctx, "register:"+email, 5, time.Hour); e != nil {
		return e
	}
	hash, e := security.Password(password, s.Config.Pepper)
	if e != nil {
		return Invalid(e.Error())
	}
	id := security.Random("usr_", 18)
	return s.transact(ctx, func(tx *sql.Tx) error {
		result, e := tx.ExecContext(ctx, `INSERT INTO users(id,email,name,password_hash,role) VALUES($1,$2,$3,$4,'customer') ON CONFLICT(email) DO NOTHING`, id, email, name, hash)
		if e != nil {
			return e
		}
		n, e := result.RowsAffected()
		if e != nil {
			return e
		}
		if n == 0 {
			return nil
		}
		if e = exec(tx, ctx, `INSERT INTO accounts(id,owner_id,kind,currency) VALUES($1,$2,'wallet','NGN')`, "wallet:"+id, id); e != nil {
			return e
		}
		if e = exec(tx, ctx, `INSERT INTO preferences(owner_id) VALUES($1)`, id); e != nil {
			return e
		}
		if e = s.createChallenge(ctx, tx, id, email, "verify"); e != nil {
			return e
		}
		return s.audit(ctx, tx, id, "customer.registered", id, map[string]any{})
	})
}
func (s *Service) createChallenge(ctx context.Context, tx *sql.Tx, id, email, purpose string) error {
	if e := exec(tx, ctx, `UPDATE challenges SET consumed=true WHERE user_id=$1 AND purpose=$2 AND NOT consumed`, id, purpose); e != nil {
		return e
	}
	challenge := security.Random("ch_", 18)
	code := security.Code()
	now := s.Now()
	if e := exec(tx, ctx, `INSERT INTO challenges(id,user_id,purpose,code_hash,expires_at,created_at) VALUES($1,$2,$3,$4,$5,$6)`, challenge, id, purpose, security.MAC(s.Config.Pepper, challenge+":"+code), now.Add(10*time.Minute), now); e != nil {
		return e
	}
	workflow := "identity-verify-email"
	if purpose == "reset" {
		workflow = "identity-password-reset"
	}
	return s.notify(ctx, tx, id, workflow, challenge, map[string]any{"code": code}, true)
}
func (s *Service) RequestChallenge(ctx context.Context, email, purpose string) error {
	email, e := normalEmail(email)
	if e != nil {
		return e
	}
	if purpose != "verify" && purpose != "reset" {
		return Invalid("unsupported challenge purpose")
	}
	if e = s.Rate(ctx, "challenge:"+email, 5, 15*time.Minute); e != nil {
		return e
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		var id string
		var verified bool
		e := tx.QueryRowContext(ctx, `SELECT id,verified FROM users WHERE email=$1 AND role='customer' AND status<>'closed' FOR UPDATE`, email).Scan(&id, &verified)
		if errors.Is(e, sql.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		if purpose == "verify" && verified {
			return nil
		}
		return s.createChallenge(ctx, tx, id, email, purpose)
	})
}
func (s *Service) VerifyChallenge(ctx context.Context, email, purpose, code, password string) error {
	email, e := normalEmail(email)
	if e != nil {
		return e
	}
	if purpose != "verify" && purpose != "reset" {
		return Invalid("unsupported challenge purpose")
	}
	if len(code) != 6 || strings.Trim(code, "0123456789") != "" {
		return Invalid("six-digit code required")
	}
	if e = s.Rate(ctx, "challenge-verify:"+email, 12, 15*time.Minute); e != nil {
		return e
	}
	var hash string
	if purpose == "reset" {
		hash, e = security.Password(password, s.Config.Pepper)
		if e != nil {
			return Invalid(e.Error())
		}
	}
	accepted := false
	e = s.transact(ctx, func(tx *sql.Tx) error {
		accepted = false
		var uid, id, stored string
		var expires time.Time
		var attempts int
		e := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE email=$1 AND role='customer' AND status<>'closed' FOR UPDATE`, email).Scan(&uid)
		if errors.Is(e, sql.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		e = tx.QueryRowContext(ctx, `SELECT id,code_hash,expires_at,attempts FROM challenges WHERE user_id=$1 AND purpose=$2 AND NOT consumed ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, uid, purpose).Scan(&id, &stored, &expires, &attempts)
		if errors.Is(e, sql.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		if attempts >= 5 || !expires.After(s.Now()) {
			return exec(tx, ctx, `UPDATE challenges SET consumed=true WHERE id=$1`, id)
		}
		if !security.Equal(stored, security.MAC(s.Config.Pepper, id+":"+code)) {
			return exec(tx, ctx, `UPDATE challenges SET attempts=attempts+1 WHERE id=$1`, id)
		}
		if e = exec(tx, ctx, `UPDATE challenges SET consumed=true WHERE id=$1`, id); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE notification_intents SET payload_enc='',state='suppressed' WHERE owner_id=$1 AND reference=$2 AND secret`, uid, id); e != nil {
			return e
		}
		if purpose == "verify" {
			if e = exec(tx, ctx, `UPDATE users SET verified=true,version=version+1 WHERE id=$1`, uid); e != nil {
				return e
			}
			e = s.notify(ctx, tx, uid, "identity-welcome", id, nil, false)
		} else {
			if e = exec(tx, ctx, `UPDATE users SET password_hash=$2,version=version+1 WHERE id=$1`, uid, hash); e != nil {
				return e
			}
			if e = exec(tx, ctx, `UPDATE sessions SET revoked=true WHERE user_id=$1`, uid); e != nil {
				return e
			}
			e = s.notify(ctx, tx, uid, "identity-password-changed", id, nil, false)
		}
		if e != nil {
			return e
		}
		accepted = true
		return s.audit(ctx, tx, uid, "challenge."+purpose, uid, map[string]any{})
	})
	if e != nil {
		return e
	}
	if !accepted {
		return unauthorized()
	}
	return nil
}

type LoginInput struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	MFACode      string `json:"mfa_code,omitempty"`
	RecoveryCode string `json:"recovery_code,omitempty"`
	Client       string `json:"client"`
	Device       string `json:"device"`
}

func (s *Service) Login(ctx context.Context, in LoginInput, audience string) (Session, error) {
	var out Session
	email, e := normalEmail(in.Email)
	if e != nil {
		return out, unauthorized()
	}
	if in.Client != "web" && in.Client != "mobile" {
		return out, Invalid("client must be web or mobile")
	}
	if !safeText(in.Device, 100) {
		return out, Invalid("device name required")
	}
	if e = s.Rate(ctx, "login:"+email, 10, 15*time.Minute); e != nil {
		return out, e
	}
	var id, hash string
	e = s.DB.QueryRowContext(ctx, `SELECT id,password_hash FROM users WHERE email=$1`, email).Scan(&id, &hash)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return out, e
	}
	if id == "" {
		hash = "$argon2id$v=19$m=32768,t=3,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	verified := security.Verify(in.Password, hash, s.Config.Pepper)
	if id == "" || !verified {
		return out, unauthorized()
	}
	e = s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.user(ctx, tx, id, true)
		if e != nil {
			return e
		}
		if u.Status == "closed" || (audience == "customer") != (u.Role == "customer") {
			return unauthorized()
		}
		var current string
		if e = tx.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id=$1`, id).Scan(&current); e != nil {
			return e
		}
		if current != hash {
			return unauthorized()
		}
		if u.MFA {
			if in.RecoveryCode != "" {
				result, e := tx.ExecContext(ctx, `UPDATE mfa_recovery_codes SET consumed=true WHERE user_id=$1 AND code_hash=$2 AND NOT consumed`, id, security.MAC(s.Config.Pepper, "recovery:"+in.RecoveryCode))
				if e != nil {
					return e
				}
				n, _ := result.RowsAffected()
				if n != 1 {
					return unauthorized()
				}
			} else if e = s.checkTOTP(ctx, tx, id, in.MFACode); e != nil {
				return e
			}
		}
		out, e = s.newSession(ctx, tx, u, in.Client, in.Device, audience, u.MFA || audience == "customer")
		if e != nil {
			return e
		}
		if audience == "customer" {
			if e = s.notify(ctx, tx, id, "security-new-device", security.Random("login_", 18), nil, false); e != nil {
				return e
			}
		}
		return s.audit(ctx, tx, id, "session.created", id, map[string]any{"audience": audience, "client": in.Client})
	})
	return out, e
}
func (s *Service) newSession(ctx context.Context, tx *sql.Tx, u User, client, device, audience string, mfa bool) (Session, error) {
	now := s.Now()
	out := Session{AccessToken: security.Random("", 32), RefreshToken: security.Random("", 32), CSRFToken: security.Random("", 32), ExpiresAt: now.Add(20 * time.Minute), RefreshExpiresAt: now.Add(30 * 24 * time.Hour), User: u, MFARequired: !mfa}
	id := security.Random("ses_", 18)
	csrfEnc, e := s.Config.Box.Seal(out.CSRFToken, "csrf:"+id)
	if e != nil {
		return Session{}, e
	}
	e = exec(tx, ctx, `INSERT INTO sessions(id,user_id,access_hash,audience,client,csrf_hash,device_name,mfa_ready,expires_at,refresh_expires_at,csrf_enc) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, id, u.ID, security.Digest(out.AccessToken), audience, client, security.Digest(out.CSRFToken), device, mfa, out.ExpiresAt, out.RefreshExpiresAt, csrfEnc)
	if e != nil {
		return Session{}, e
	}
	e = exec(tx, ctx, `INSERT INTO refresh_tokens(token_hash,session_id,expires_at) VALUES($1,$2,$3)`, security.Digest(out.RefreshToken), id, out.RefreshExpiresAt)
	return out, e
}
func (s *Service) Authenticate(ctx context.Context, token string) (Principal, error) {
	var p Principal
	var uid string
	var expires time.Time
	var revoked bool
	if len(token) != 43 {
		return p, unauthorized()
	}
	e := s.DB.QueryRowContext(ctx, `SELECT id,user_id,audience,client,csrf_hash,mfa_ready,expires_at,revoked FROM sessions WHERE access_hash=$1`, security.Digest(token)).Scan(&p.SessionID, &uid, &p.Audience, &p.Client, &p.CSRFHash, &p.MFAReady, &expires, &revoked)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return p, e
	}
	if errors.Is(e, sql.ErrNoRows) || revoked || !expires.After(s.Now()) {
		return p, unauthorized()
	}
	if e != nil {
		return p, e
	}
	p.User, e = s.user(ctx, s.DB, uid, false)
	if e != nil {
		return p, e
	}
	if p.User.Status == "closed" {
		return p, unauthorized()
	}
	if (p.Audience == "customer") != (p.User.Role == "customer") {
		return p, unauthorized()
	}
	return p, nil
}
func (s *Service) activePrincipal(ctx context.Context, tx *sql.Tx, p Principal) (User, error) {
	u, e := s.user(ctx, tx, p.User.ID, true)
	if e != nil {
		return u, e
	}
	var ok bool
	e = tx.QueryRowContext(ctx, `SELECT NOT revoked AND expires_at>$2 AND audience=$3 AND user_id=$4 FROM sessions WHERE id=$1 FOR UPDATE`, p.SessionID, s.Now(), p.Audience, u.ID).Scan(&ok)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return u, e
	}
	if errors.Is(e, sql.ErrNoRows) || !ok || u.Status == "closed" {
		return u, unauthorized()
	}
	if e != nil {
		return u, e
	}
	return u, nil
}
func (s *Service) Refresh(ctx context.Context, token, csrf, client, expectedAudience string) (Session, error) {
	var out Session
	if len(token) != 43 {
		return out, unauthorized()
	}
	replayed := false
	e := s.transact(ctx, func(tx *sql.Tx) error {
		replayed = false
		var id, uid, storedCSRF, audience, storedClient string
		var consumed, revoked, mfa bool
		var expires time.Time
		e := tx.QueryRowContext(ctx, `SELECT t.session_id,t.consumed,s.user_id,s.csrf_hash,s.audience,s.client,s.revoked,s.mfa_ready,s.refresh_expires_at FROM refresh_tokens t JOIN sessions s ON s.id=t.session_id WHERE t.token_hash=$1 FOR UPDATE OF s,t`, security.Digest(token)).Scan(&id, &consumed, &uid, &storedCSRF, &audience, &storedClient, &revoked, &mfa, &expires)
		if errors.Is(e, sql.ErrNoRows) {
			return unauthorized()
		}
		if e != nil {
			return e
		}
		if revoked || !expires.After(s.Now()) || client != storedClient || audience != expectedAudience {
			return unauthorized()
		}
		if consumed {
			replayed = true
			return exec(tx, ctx, `UPDATE sessions SET revoked=true WHERE id=$1`, id)
		}
		if client == "web" && !security.Equal(storedCSRF, security.Digest(csrf)) {
			return denied()
		}
		u, e := s.user(ctx, tx, uid, false)
		if e != nil {
			return e
		}
		if u.Status == "closed" {
			return unauthorized()
		}
		out = Session{AccessToken: security.Random("", 32), RefreshToken: security.Random("", 32), CSRFToken: security.Random("", 32), ExpiresAt: s.Now().Add(20 * time.Minute), RefreshExpiresAt: expires, User: u, MFARequired: !mfa}
		if e = exec(tx, ctx, `UPDATE refresh_tokens SET consumed=true WHERE token_hash=$1`, security.Digest(token)); e != nil {
			return e
		}
		csrfEnc, e := s.Config.Box.Seal(out.CSRFToken, "csrf:"+id)
		if e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE sessions SET access_hash=$2,csrf_hash=$3,expires_at=$4,csrf_enc=$5 WHERE id=$1`, id, security.Digest(out.AccessToken), security.Digest(out.CSRFToken), out.ExpiresAt, csrfEnc); e != nil {
			return e
		}
		return exec(tx, ctx, `INSERT INTO refresh_tokens(token_hash,session_id,expires_at) VALUES($1,$2,$3)`, security.Digest(out.RefreshToken), id, expires)
	})
	if replayed {
		return Session{}, unauthorized()
	}
	return out, e
}
func (s *Service) RevokeSession(ctx context.Context, p Principal, id string) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		result, e := tx.ExecContext(ctx, `UPDATE sessions SET revoked=true WHERE id=$1 AND user_id=$2`, id, p.User.ID)
		if e != nil {
			return e
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			return missing()
		}
		return s.audit(ctx, tx, p.User.ID, "session.revoked", id, map[string]any{})
	})
}
func (s *Service) Sessions(ctx context.Context, p Principal) ([]map[string]any, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id,device_name,client,created_at,expires_at,revoked FROM sessions WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`, p.User.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, device, client string
		var created, expires time.Time
		var revoked bool
		if e = rows.Scan(&id, &device, &client, &created, &expires, &revoked); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": id, "device": device, "client": client, "created_at": created, "expires_at": expires, "revoked": revoked, "current": id == p.SessionID})
	}
	return out, rows.Err()
}
func (s *Service) checkTOTP(ctx context.Context, tx *sql.Tx, id, code string) error {
	var encrypted string
	var last int64
	e := tx.QueryRowContext(ctx, `SELECT mfa_secret,mfa_counter FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&encrypted, &last)
	if e != nil {
		return e
	}
	secret, e := s.Config.Box.Open(encrypted, "mfa:"+id)
	if e != nil {
		return unauthorized()
	}
	counter, ok := security.VerifyTOTP(secret, code, s.Now(), last)
	if !ok {
		return unauthorized()
	}
	return exec(tx, ctx, `UPDATE users SET mfa_counter=$2 WHERE id=$1`, id, counter)
}
func (s *Service) reauthenticate(ctx context.Context, tx *sql.Tx, u User, password, code string) error {
	var hash string
	if e := tx.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id=$1`, u.ID).Scan(&hash); e != nil {
		return e
	}
	if !security.Verify(password, hash, s.Config.Pepper) {
		return unauthorized()
	}
	if u.MFA {
		return s.checkTOTP(ctx, tx, u.ID, code)
	}
	return nil
}
func (s *Service) MFAStart(ctx context.Context, p Principal, password, code string) (map[string]string, error) {
	if e := s.Rate(ctx, "sensitive:"+p.User.ID, 6, 15*time.Minute); e != nil {
		return nil, e
	}
	secret := security.NewTOTPSecret()
	sealed, e := s.Config.Box.Seal(secret, "mfa:"+p.User.ID)
	if e != nil {
		return nil, e
	}
	e = s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if e = s.reauthenticate(ctx, tx, u, password, code); e != nil {
			return e
		}
		return exec(tx, ctx, `UPDATE users SET mfa_pending=$2,mfa_pending_expires=$3 WHERE id=$1`, u.ID, sealed, s.Now().Add(10*time.Minute))
	})
	if e != nil {
		return nil, e
	}
	return map[string]string{"secret": secret, "otpauth_url": "otpauth://totp/" + url.PathEscape("Qpay-Fintech:"+p.User.Email) + "?secret=" + secret + "&issuer=Qpay-Fintech&algorithm=SHA1&digits=6&period=30"}, nil
}
func (s *Service) MFAConfirm(ctx context.Context, p Principal, code string) ([]string, error) {
	if e := s.Rate(ctx, "mfa-confirm:"+p.User.ID, 6, 15*time.Minute); e != nil {
		return nil, e
	}
	codes := []string{}
	for i := 0; i < 10; i++ {
		codes = append(codes, security.Random("", 15))
	}
	e := s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		var sealed string
		var pendingExpiry sql.NullTime
		if e = tx.QueryRowContext(ctx, `SELECT mfa_pending,mfa_pending_expires FROM users WHERE id=$1`, u.ID).Scan(&sealed, &pendingExpiry); e != nil {
			return e
		}
		if !pendingExpiry.Valid || !pendingExpiry.Time.After(s.Now()) {
			return unauthorized()
		}
		secret, e := s.Config.Box.Open(sealed, "mfa:"+u.ID)
		if e != nil {
			return unauthorized()
		}
		counter, ok := security.VerifyTOTP(secret, code, s.Now(), -1)
		if !ok {
			return unauthorized()
		}
		if e = exec(tx, ctx, `UPDATE users SET mfa_secret=mfa_pending,mfa_pending='',mfa_pending_expires=NULL,mfa_enabled=true,mfa_counter=$2,version=version+1 WHERE id=$1`, u.ID, counter); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE sessions SET revoked=true WHERE user_id=$1 AND id<>$2`, u.ID, p.SessionID); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE sessions SET mfa_ready=true WHERE id=$1`, p.SessionID); e != nil {
			return e
		}
		if e = exec(tx, ctx, `DELETE FROM mfa_recovery_codes WHERE user_id=$1`, u.ID); e != nil {
			return e
		}
		for _, value := range codes {
			if e = exec(tx, ctx, `INSERT INTO mfa_recovery_codes(user_id,code_hash) VALUES($1,$2)`, u.ID, security.MAC(s.Config.Pepper, "recovery:"+value)); e != nil {
				return e
			}
		}
		if u.Role == "customer" {
			if e = s.notify(ctx, tx, u.ID, "security-mfa-enabled", security.Random("mfa_", 18), nil, false); e != nil {
				return e
			}
		}
		return s.audit(ctx, tx, u.ID, "mfa.enabled", u.ID, map[string]any{})
	})
	if e != nil {
		return nil, e
	}
	return codes, nil
}
func (s *Service) ChangeCredential(ctx context.Context, p Principal, kind, password, code, newValue string) error {
	if e := s.Rate(ctx, "sensitive:"+p.User.ID, 6, 15*time.Minute); e != nil {
		return e
	}
	var hash string
	var e error
	if kind == "password" {
		hash, e = security.Password(newValue, s.Config.Pepper)
	} else if kind == "pin" {
		hash, e = security.PIN(newValue, s.Config.Pepper)
	} else {
		return Invalid("unsupported credential")
	}
	if e != nil {
		return Invalid(e.Error())
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if e = s.reauthenticate(ctx, tx, u, password, code); e != nil {
			return e
		}
		column := "password_hash"
		workflow := "identity-password-changed"
		if kind == "pin" {
			column = "pin_hash"
			workflow = "security-pin-changed"
			if e = s.eligible(u); e != nil {
				return e
			}
		}
		if e = exec(tx, ctx, `UPDATE users SET `+column+`=$2,version=version+1 WHERE id=$1`, u.ID, hash); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE sessions SET revoked=true WHERE user_id=$1`, u.ID); e != nil {
			return e
		}
		if u.Role == "customer" {
			if e = s.notify(ctx, tx, u.ID, workflow, security.Random("change_", 18), nil, false); e != nil {
				return e
			}
		}
		return s.audit(ctx, tx, u.ID, kind+".changed", u.ID, map[string]any{})
	})
}
func (s *Service) UpdateName(ctx context.Context, p Principal, name string) (User, error) {
	if !safeText(name, 80) {
		return User{}, Invalid("invalid name")
	}
	e := s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE users SET name=$2,version=version+1 WHERE id=$1`, u.ID, name); e != nil {
			return e
		}
		return s.audit(ctx, tx, u.ID, "profile.updated", u.ID, map[string]any{})
	})
	if e != nil {
		return User{}, e
	}
	return s.user(ctx, s.DB, p.User.ID, false)
}
