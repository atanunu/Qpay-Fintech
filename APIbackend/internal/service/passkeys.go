package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

type PasskeyProvider = webauthn.WebAuthn
type credentialOwner struct {
	User        User
	Handle      []byte
	Credentials []webauthn.Credential
}

func (u credentialOwner) WebAuthnID() []byte                         { return u.Handle }
func (u credentialOwner) WebAuthnName() string                       { return u.User.Email }
func (u credentialOwner) WebAuthnDisplayName() string                { return u.User.Name }
func (u credentialOwner) WebAuthnCredentials() []webauthn.Credential { return u.Credentials }
func (s *Service) passkeyUser(ctx context.Context, tx *sql.Tx, id string, create bool) (credentialOwner, error) {
	var out credentialOwner
	u, e := s.user(ctx, tx, id, true)
	if e != nil {
		return out, e
	}
	if u.Role != "customer" || u.Status == "closed" {
		return out, unauthorized()
	}
	out.User = u
	rp := s.Config.Passkeys.Config.RPID
	if create {
		raw := make([]byte, 32)
		if _, e = rand.Read(raw); e != nil {
			return out, e
		}
		if e = exec(tx, ctx, `INSERT INTO webauthn_users(owner_id,rp_id,handle) VALUES($1,$2,$3) ON CONFLICT(owner_id,rp_id) DO NOTHING`, id, rp, raw); e != nil {
			return out, e
		}
	}
	if e = tx.QueryRowContext(ctx, `SELECT handle FROM webauthn_users WHERE owner_id=$1 AND rp_id=$2`, id, rp).Scan(&out.Handle); e != nil {
		return out, unauthorized()
	}
	rows, e := tx.QueryContext(ctx, `SELECT id,credential_enc FROM webauthn_credentials WHERE owner_id=$1 AND rp_id=$2 AND NOT revoked ORDER BY id`, id, rp)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var cid, sealed string
		if e = rows.Scan(&cid, &sealed); e != nil {
			return out, e
		}
		raw, e := s.Config.Box.Open(sealed, "passkey:"+id+":"+cid)
		if e != nil {
			return out, e
		}
		var c webauthn.Credential
		if e = json.Unmarshal([]byte(raw), &c); e != nil {
			return out, e
		}
		out.Credentials = append(out.Credentials, c)
	}
	return out, rows.Err()
}
func (s *Service) PasskeyBegin(ctx context.Context, p Principal, origin, binding, password, code string, register bool) (map[string]any, error) {
	if s.Config.Passkeys == nil {
		return nil, &Fault{503, "passkeys_unavailable", "passkey relying-party configuration is not enabled"}
	}
	if len(binding) != 43 {
		return nil, unauthorized()
	}
	if register {
		if e := p.Customer(); e != nil {
			return nil, e
		}
		if e := s.Rate(ctx, "passkey-enrol:"+p.User.ID, 6, 15*time.Minute); e != nil {
			return nil, e
		}
	}
	var options any
	id := security.Random("pkc_", 24)
	kind := "login"
	if register {
		kind = "register"
	}
	e := s.transact(ctx, func(tx *sql.Tx) error {
		var data *webauthn.SessionData
		var owner, session any
		var e error
		if register {
			u, e := s.activePrincipal(ctx, tx, p)
			if e != nil {
				return e
			}
			if !u.Verified || u.Status != "active" {
				return denied()
			}
			if e = s.reauthenticate(ctx, tx, u, password, code); e != nil {
				return e
			}
			pu, e := s.passkeyUser(ctx, tx, u.ID, true)
			if e != nil {
				return e
			}
			if len(pu.Credentials) >= 10 {
				return Invalid("maximum ten active passkeys")
			}
			options, data, e = s.Config.Passkeys.BeginRegistration(pu, webauthn.WithRegistrationOrigin(origin), webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired))
			owner = u.ID
			session = p.SessionID
		} else {
			options, data, e = s.Config.Passkeys.BeginDiscoverableLogin(webauthn.WithLoginOrigin(origin), webauthn.WithUserVerification(protocol.VerificationRequired))
		}
		if e != nil {
			return Invalid("passkey ceremony could not start for this origin")
		}
		encoded, e := s.Config.Box.Seal(jsonText(data), "passkey-ceremony:"+id)
		if e != nil {
			return e
		}
		return exec(tx, ctx, `INSERT INTO webauthn_ceremonies(id,owner_id,session_id,kind,binding_hash,data_enc,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, owner, session, kind, security.Digest(binding), encoded, s.Now().Add(3*time.Minute))
	})
	return map[string]any{"id": id, "options": options, "expires_at": s.Now().Add(3 * time.Minute)}, e
}

type PasskeyFinishInput struct {
	ID         string          `json:"id"`
	Credential json.RawMessage `json:"credential"`
	Label      string          `json:"label,omitempty"`
	MFACode    string          `json:"mfa_code,omitempty"`
}

func (s *Service) PasskeyFinish(ctx context.Context, p Principal, binding string, in PasskeyFinishInput, register bool) (Session, error) {
	var out Session
	if s.Config.Passkeys == nil {
		return out, unavailable()
	}
	if len(binding) != 43 || len(in.Credential) > 65536 || len(in.Credential) < 20 {
		return out, Invalid("invalid passkey response")
	}
	if register && !safeText(in.Label, 80) {
		return out, Invalid("passkey label required")
	}
	if e := s.Rate(ctx, "passkey-finish:"+in.ID, 6, 5*time.Minute); e != nil {
		return out, e
	}
	var invalid error
	e := s.transact(ctx, func(tx *sql.Tx) error {
		invalid = nil
		var owner, session sql.NullString
		var kind, hash, sealed string
		var expires time.Time
		var consumed bool
		e := tx.QueryRowContext(ctx, `SELECT owner_id,session_id,kind,binding_hash,data_enc,expires_at,consumed FROM webauthn_ceremonies WHERE id=$1 FOR UPDATE`, in.ID).Scan(&owner, &session, &kind, &hash, &sealed, &expires, &consumed)
		if errors.Is(e, sql.ErrNoRows) {
			return unauthorized()
		}
		if e != nil {
			return e
		}
		if consumed || !expires.After(s.Now()) || !security.Equal(hash, security.Digest(binding)) || (register != (kind == "register")) {
			return unauthorized()
		}
		if register && (owner.String != p.User.ID || session.String != p.SessionID) {
			return unauthorized()
		}
		// Consume even a cryptographically invalid response. Retrying requires a fresh challenge.
		if e = exec(tx, ctx, `UPDATE webauthn_ceremonies SET consumed=true,data_enc='' WHERE id=$1`, in.ID); e != nil {
			return e
		}
		raw, e := s.Config.Box.Open(sealed, "passkey-ceremony:"+in.ID)
		if e != nil {
			return e
		}
		var data webauthn.SessionData
		if e = json.Unmarshal([]byte(raw), &data); e != nil {
			return e
		}
		request, e := http.NewRequestWithContext(ctx, "POST", "https://passkey.invalid", bytes.NewReader(in.Credential))
		if e != nil {
			return e
		}
		request.Header.Set("Content-Type", "application/json")
		var credential *webauthn.Credential
		var user credentialOwner
		if register {
			if _, e = s.activePrincipal(ctx, tx, p); e != nil {
				return e
			}
			user, e = s.passkeyUser(ctx, tx, p.User.ID, false)
			if e != nil {
				return e
			}
			credential, e = s.Config.Passkeys.FinishRegistration(user, data, request)
		} else {
			var resolved webauthn.User
			resolved, credential, e = s.Config.Passkeys.FinishPasskeyLogin(func(rawID, handle []byte) (webauthn.User, error) {
				var uid string
				err := tx.QueryRowContext(ctx, `SELECT u.owner_id FROM webauthn_users u JOIN webauthn_credentials c ON c.owner_id=u.owner_id AND c.rp_id=u.rp_id WHERE u.rp_id=$1 AND u.handle=$2 AND c.credential_id=$3 AND NOT c.revoked`, s.Config.Passkeys.Config.RPID, handle, rawID).Scan(&uid)
				if err != nil {
					return nil, unauthorized()
				}
				pu, err := s.passkeyUser(ctx, tx, uid, false)
				return pu, err
			}, data, request)
			if e == nil {
				var ok bool
				user, ok = resolved.(credentialOwner)
				if !ok {
					return errors.New("unexpected passkey owner")
				}
			}
		}
		if e != nil || credential == nil || !credential.Flags.UserVerified || credential.Authenticator.CloneWarning {
			invalid = unauthorized()
			return nil
		}
		if user.User.Status != "active" || !user.User.Verified {
			invalid = unauthorized()
			return nil
		}
		if !register && user.User.MFA {
			if e = s.checkTOTP(ctx, tx, user.User.ID, in.MFACode); e != nil {
				invalid = unauthorized()
				return nil
			}
		}
		rp := s.Config.Passkeys.Config.RPID
		if register {
			cid := security.Random("passkey_", 18)
			encrypted, e := s.Config.Box.Seal(jsonText(credential), "passkey:"+user.User.ID+":"+cid)
			if e != nil {
				return e
			}
			if len(user.Credentials) >= 10 {
				return Invalid("maximum ten active passkeys")
			}
			if e = exec(tx, ctx, `INSERT INTO webauthn_credentials(id,owner_id,rp_id,credential_id,credential_enc,label) VALUES($1,$2,$3,$4,$5,$6)`, cid, user.User.ID, rp, credential.ID, encrypted, in.Label); e != nil {
				return e
			}
			if e = s.notify(ctx, tx, user.User.ID, "security-passkey-added", cid, nil, false); e != nil {
				return e
			}
			return s.audit(ctx, tx, user.User.ID, "passkey.registered", cid, map[string]any{})
		}
		var cid string
		if e = tx.QueryRowContext(ctx, `SELECT id FROM webauthn_credentials WHERE owner_id=$1 AND rp_id=$2 AND credential_id=$3 AND NOT revoked FOR UPDATE`, user.User.ID, rp, credential.ID).Scan(&cid); e != nil {
			return unauthorized()
		}
		encrypted, e := s.Config.Box.Seal(jsonText(credential), "passkey:"+user.User.ID+":"+cid)
		if e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE webauthn_credentials SET credential_enc=$2,last_used_at=$3 WHERE id=$1`, cid, encrypted, s.Now()); e != nil {
			return e
		}
		out, e = s.newSession(ctx, tx, user.User, "web", "Qpay Web passkey", "customer", true)
		if e != nil {
			return e
		}
		if e = s.notify(ctx, tx, user.User.ID, "security-new-device", in.ID, nil, false); e != nil {
			return e
		}
		return s.audit(ctx, tx, user.User.ID, "passkey.login", cid, map[string]any{})
	})
	if e != nil {
		return Session{}, e
	}
	if invalid != nil {
		return Session{}, invalid
	}
	return out, nil
}
func (s *Service) PasskeyList(ctx context.Context, p Principal) ([]map[string]any, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id,label,created_at,last_used_at,revoked FROM webauthn_credentials WHERE owner_id=$1 ORDER BY created_at DESC LIMIT 100`, p.User.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, label string
		var created time.Time
		var last sql.NullTime
		var revoked bool
		if e = rows.Scan(&id, &label, &created, &last, &revoked); e != nil {
			return nil, e
		}
		var used any
		if last.Valid {
			used = last.Time
		}
		out = append(out, map[string]any{"id": id, "label": label, "created_at": created, "last_used_at": used, "revoked": revoked})
	}
	return out, rows.Err()
}
func (s *Service) RevokePasskey(ctx context.Context, p Principal, id, password, code string) error {
	if e := s.Rate(ctx, "sensitive:"+p.User.ID, 6, 15*time.Minute); e != nil {
		return e
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if e = s.reauthenticate(ctx, tx, u, password, code); e != nil {
			return e
		}
		r, e := tx.ExecContext(ctx, `UPDATE webauthn_credentials SET revoked=true WHERE id=$1 AND owner_id=$2 AND NOT revoked`, id, u.ID)
		if e != nil {
			return e
		}
		n, _ := r.RowsAffected()
		if n != 1 {
			return missing()
		}
		if e = exec(tx, ctx, `UPDATE sessions SET revoked=true WHERE user_id=$1 AND id<>$2`, u.ID, p.SessionID); e != nil {
			return e
		}
		return s.audit(ctx, tx, u.ID, "passkey.revoked", id, map[string]any{})
	})
}
