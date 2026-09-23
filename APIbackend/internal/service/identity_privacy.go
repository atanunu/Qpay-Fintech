package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
)

type IdentityInput struct {
	LegalName    string   `json:"legal_name"`
	DateOfBirth  string   `json:"date_of_birth"`
	Address      string   `json:"address"`
	DocumentType string   `json:"document_type"`
	Uploads      []string `json:"upload_ids"`
	Consent      bool     `json:"consent"`
	Version      int64    `json:"version"`
}

func (s *Service) IdentityDraft(ctx context.Context, p Principal) (map[string]any, error) {
	var sealed string
	var version int64
	var submitted sql.NullString
	e := s.DB.QueryRowContext(ctx, `SELECT content_enc,version,submitted_case FROM identity_drafts WHERE owner_id=$1`, p.User.ID).Scan(&sealed, &version, &submitted)
	if errors.Is(e, sql.ErrNoRows) {
		return map[string]any{"version": 0, "draft": nil}, nil
	}
	if e != nil {
		return nil, e
	}
	raw, e := s.Config.Box.Open(sealed, "identity-draft:"+p.User.ID)
	if e != nil {
		return nil, e
	}
	var draft IdentityInput
	if e = json.Unmarshal([]byte(raw), &draft); e != nil {
		return nil, e
	}
	draft.Version = version
	return map[string]any{"version": version, "draft": draft, "submitted_case": submitted.String}, nil
}
func (s *Service) validateIdentity(in IdentityInput, complete bool) error {
	if len(in.LegalName) > 150 || len(in.Address) > 500 || len(in.DateOfBirth) > 10 || len(in.Uploads) > 4 {
		return Invalid("identity fields exceed accepted limits")
	}
	if complete {
		if !safeText(in.LegalName, 150) || !safeText(in.Address, 500) || !in.Consent || len(in.Uploads) == 0 {
			return Invalid("legal name, date of birth, address, documents and consent are required")
		}
		dob, e := time.Parse("2006-01-02", in.DateOfBirth)
		if e != nil || dob.After(s.Now().AddDate(-18, 0, 0)) || dob.Before(s.Now().AddDate(-120, 0, 0)) {
			return Invalid("this account is for adults aged 18 or over")
		}
		switch in.DocumentType {
		case "national-id", "passport", "drivers-licence", "voter-card":
		default:
			return Invalid("choose an accepted identity document")
		}
	}
	return nil
}
func (s *Service) SaveIdentityDraft(ctx context.Context, p Principal, in IdentityInput) (map[string]any, error) {
	if e := s.validateIdentity(in, false); e != nil {
		return nil, e
	}
	sealed, e := s.Config.Box.Seal(jsonText(in), "identity-draft:"+p.User.ID)
	if e != nil {
		return nil, e
	}
	e = s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if !u.Verified || u.Status != "active" {
			return denied()
		}
		var version int64
		var submitted sql.NullString
		e = tx.QueryRowContext(ctx, `SELECT version,submitted_case FROM identity_drafts WHERE owner_id=$1 FOR UPDATE`, u.ID).Scan(&version, &submitted)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		if version != in.Version {
			return conflict("identity draft changed; reload before saving")
		}
		if submitted.Valid {
			var state string
			if e = tx.QueryRowContext(ctx, `SELECT status FROM kyc_cases WHERE id=$1`, submitted.String).Scan(&state); e != nil {
				return e
			}
			if state == "submitted" || state == "approved" {
				return conflict("submitted evidence cannot be overwritten; use the review process")
			}
		}
		return exec(tx, ctx, `INSERT INTO identity_drafts(owner_id,content_enc) VALUES($1,$2) ON CONFLICT(owner_id) DO UPDATE SET content_enc=excluded.content_enc,version=identity_drafts.version+1,submitted_case=NULL,updated_at=now()`, u.ID, sealed)
	})
	if e != nil {
		return nil, e
	}
	return s.IdentityDraft(ctx, p)
}
func (s *Service) SubmitIdentity(ctx context.Context, p Principal, version int64) (string, error) {
	id := stringID("kyc_")
	e := s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if !u.Verified || u.Status != "active" {
			return denied()
		}
		var sealed string
		var stored int64
		var prior sql.NullString
		if e = tx.QueryRowContext(ctx, `SELECT content_enc,version,submitted_case FROM identity_drafts WHERE owner_id=$1 FOR UPDATE`, u.ID).Scan(&sealed, &stored, &prior); e != nil {
			return isMissing(e)
		}
		if prior.Valid {
			id = prior.String
			return nil
		}
		if version != stored {
			return conflict("draft version changed")
		}
		raw, e := s.Config.Box.Open(sealed, "identity-draft:"+u.ID)
		if e != nil {
			return e
		}
		var in IdentityInput
		if e = json.Unmarshal([]byte(raw), &in); e != nil {
			return e
		}
		if e = s.validateIdentity(in, true); e != nil {
			return e
		}
		var open bool
		if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM kyc_cases WHERE owner_id=$1 AND status IN('submitted','information_required'))`, u.ID).Scan(&open); e != nil {
			return e
		}
		if open {
			return conflict("an identity case is already open")
		}
		seen := map[string]bool{}
		for _, upload := range in.Uploads {
			if seen[upload] {
				return Invalid("duplicate document")
			}
			seen[upload] = true
			var state string
			if e = tx.QueryRowContext(ctx, `SELECT state FROM private_uploads WHERE id=$1 AND owner_id=$2 AND purpose='kyc' FOR UPDATE`, upload, u.ID).Scan(&state); e != nil {
				return isMissing(e)
			}
			if state != "clean" && (state != "local_unscanned" || s.Config.Environment != "local") {
				return conflict("all identity documents must pass private upload checks")
			}
		}
		if e = exec(tx, ctx, `INSERT INTO kyc_cases(id,owner_id,evidence_ref) VALUES($1,$2,$3)`, id, u.ID, "digital:"+id); e != nil {
			return e
		}
		for upload := range seen {
			if e = exec(tx, ctx, `INSERT INTO kyc_documents(case_id,upload_id) VALUES($1,$2)`, id, upload); e != nil {
				return e
			}
		}
		// Immutable encrypted case snapshot preserves the identity reviewed even if a later draft changes.
		snapshot, e := s.Config.Box.Seal(jsonText(in), "identity-case:"+id)
		if e != nil {
			return e
		}
		if e = exec(tx, ctx, `INSERT INTO identity_case_snapshots(case_id,content_enc) VALUES($1,$2)`, id, snapshot); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE identity_drafts SET submitted_case=$2,version=version+1 WHERE owner_id=$1`, u.ID, id); e != nil {
			return e
		}
		if e = s.notify(ctx, tx, u.ID, "kyc-submitted", id, nil, false); e != nil {
			return e
		}
		return s.audit(ctx, tx, u.ID, "kyc.digital_submitted", id, map[string]any{"document_count": len(seen)})
	})
	return id, e
}
func (s *Service) IdentityCase(ctx context.Context, p Principal, id string) (map[string]any, error) {
	if e := requireRole(p, "admin", "compliance"); e != nil {
		return nil, e
	}
	var owner, sealed string
	if e := s.DB.QueryRowContext(ctx, `SELECT k.owner_id,i.content_enc FROM identity_case_snapshots i JOIN kyc_cases k ON k.id=i.case_id WHERE i.case_id=$1`, id).Scan(&owner, &sealed); e != nil {
		return nil, isMissing(e)
	}
	raw, e := s.Config.Box.Open(sealed, "identity-case:"+id)
	if e != nil {
		return nil, e
	}
	var input IdentityInput
	if e = json.Unmarshal([]byte(raw), &input); e != nil {
		return nil, e
	}
	e = s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		p.User = u
		if e = requireRole(p, "admin", "compliance"); e != nil {
			return e
		}
		return s.audit(ctx, tx, u.ID, "kyc.evidence_viewed", id, map[string]any{})
	})
	return map[string]any{"owner_id": owner, "identity": input}, e
}
func (s *Service) recordVerifiedIdentity(ctx context.Context, tx *sql.Tx, owner, id string) error {
	var sealed string
	e := tx.QueryRowContext(ctx, `SELECT content_enc FROM identity_case_snapshots WHERE case_id=$1`, id).Scan(&sealed)
	if errors.Is(e, sql.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	raw, e := s.Config.Box.Open(sealed, "identity-case:"+id)
	if e != nil {
		return e
	}
	var in IdentityInput
	if e = json.Unmarshal([]byte(raw), &in); e != nil {
		return e
	}
	name, e := s.Config.Box.Seal(in.LegalName, "verified-identity:"+owner)
	if e != nil {
		return e
	}
	return exec(tx, ctx, `INSERT INTO verified_identities(owner_id,legal_name_enc,case_id) VALUES($1,$2,$3) ON CONFLICT(owner_id) DO UPDATE SET legal_name_enc=excluded.legal_name_enc,case_id=excluded.case_id,verified_at=now()`, owner, name, id)
}

type ContactChangeInput struct {
	NewEmail string `json:"new_email"`
	Password string `json:"password"`
	MFACode  string `json:"mfa_code,omitempty"`
}

func (s *Service) BeginEmailChange(ctx context.Context, p Principal, in ContactChangeInput) (map[string]any, error) {
	email, e := normalEmail(in.NewEmail)
	if e != nil {
		return nil, e
	}
	if e = s.Rate(ctx, "email-change:"+p.User.ID, 3, time.Hour); e != nil {
		return nil, e
	}
	id := security.Random("contact_", 24)
	oldCode, newCode := security.Code(), security.Code()
	sealed, e := s.Config.Box.Seal(email, "contact-change:"+id)
	if e != nil {
		return nil, e
	}
	expires := s.Now().Add(10 * time.Minute)
	e = s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if !u.Verified || u.Status != "active" || email == u.Email {
			return denied()
		}
		if e = s.reauthenticate(ctx, tx, u, in.Password, in.MFACode); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE account_changes SET consumed=true WHERE owner_id=$1 AND NOT consumed`, u.ID); e != nil {
			return e
		}
		if e = exec(tx, ctx, `INSERT INTO account_changes(id,owner_id,session_id,kind,new_value_enc,old_value_hash,old_code_hash,new_code_hash,expires_at) VALUES($1,$2,$3,'email',$4,$5,$6,$7,$8)`, id, u.ID, p.SessionID, sealed, security.MAC(s.Config.Pepper, u.Email), security.MAC(s.Config.Pepper, id+":old:"+oldCode), security.MAC(s.Config.Pepper, id+":new:"+newCode), expires); e != nil {
			return e
		}
		if e = s.notify(ctx, tx, u.ID, "identity-contact-old-code", id, map[string]any{"code": oldCode}, true); e != nil {
			return e
		}
		if e = s.notifyTo(ctx, tx, u.ID, "identity-contact-new-code", id, id, map[string]any{"code": newCode}, true, email); e != nil {
			return e
		}
		return s.audit(ctx, tx, u.ID, "contact.change_requested", id, map[string]any{})
	})
	return map[string]any{"id": id, "expires_at": expires, "requires": "both_old_and_new_email_codes"}, e
}
func (s *Service) FinishEmailChange(ctx context.Context, p Principal, id, oldCode, newCode string) error {
	if len(oldCode) != 6 || len(newCode) != 6 {
		return Invalid("both six-digit codes are required")
	}
	if e := s.Rate(ctx, "contact-verify:"+p.User.ID, 10, 15*time.Minute); e != nil {
		return e
	}
	valid := false
	e := s.transact(ctx, func(tx *sql.Tx) error {
		valid = false
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		var encrypted, oldHash, newHash, oldAddress string
		var expires time.Time
		var attempts int
		e = tx.QueryRowContext(ctx, `SELECT new_value_enc,old_code_hash,new_code_hash,old_value_hash,expires_at,attempts FROM account_changes WHERE id=$1 AND owner_id=$2 AND session_id=$3 AND NOT consumed FOR UPDATE`, id, u.ID, p.SessionID).Scan(&encrypted, &oldHash, &newHash, &oldAddress, &expires, &attempts)
		if errors.Is(e, sql.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		if attempts >= 5 || !expires.After(s.Now()) || !security.Equal(oldAddress, security.MAC(s.Config.Pepper, u.Email)) {
			return nil
		}
		if !security.Equal(oldHash, security.MAC(s.Config.Pepper, id+":old:"+oldCode)) || !security.Equal(newHash, security.MAC(s.Config.Pepper, id+":new:"+newCode)) {
			return exec(tx, ctx, `UPDATE account_changes SET attempts=attempts+1 WHERE id=$1`, id)
		}
		email, e := s.Config.Box.Open(encrypted, "contact-change:"+id)
		if e != nil {
			return e
		}
		if e = exec(tx, ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, email); e != nil {
			return e
		}
		var taken bool
		if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email=$1 AND id<>$2)`, email, u.ID).Scan(&taken); e != nil {
			return e
		}
		if taken {
			return conflict("this address cannot be used for this change")
		}
		if e = exec(tx, ctx, `UPDATE account_changes SET consumed=true WHERE id=$1`, id); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE notification_intents SET state='suppressed',payload_enc='' WHERE owner_id=$1 AND reference=$2 AND secret`, u.ID, id); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE users SET email=$2,version=version+1 WHERE id=$1`, u.ID, email); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE sessions SET revoked=true WHERE user_id=$1`, u.ID); e != nil {
			return e
		}
		if e = s.notify(ctx, tx, u.ID, "identity-contact-completed", id, nil, false); e != nil {
			return e
		}
		valid = true
		return s.audit(ctx, tx, u.ID, "contact.email_changed", id, map[string]any{})
	})
	if e != nil {
		return e
	}
	if !valid {
		return unauthorized()
	}
	return nil
}
func (s *Service) ClosureEligibility(ctx context.Context, p Principal) (map[string]any, error) {
	b, e := s.Balance(ctx, p.User.ID)
	if e != nil {
		return nil, e
	}
	var pending, cases int
	if e = s.DB.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM payments WHERE (owner_id=$1 OR recipient_id=$1) AND status NOT IN('succeeded','failed')),(SELECT count(*) FROM support_cases WHERE owner_id=$1 AND status<>'resolved')`, p.User.ID).Scan(&pending, &cases); e != nil {
		return nil, e
	}
	return map[string]any{"can_close": b.Book == 0 && b.Held == 0 && pending == 0 && cases == 0 && p.User.Status == "active", "balance": b, "pending_payments": pending, "open_cases": cases, "retention": "Closing access does not erase required financial, security or dispute records."}, nil
}
func (s *Service) CloseAccount(ctx context.Context, p Principal, password, code, reason string) error {
	if !safeText(reason, 500) {
		return Invalid("closure reason required")
	}
	if e := s.Rate(ctx, "closure:"+p.User.ID, 3, time.Hour); e != nil {
		return e
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if u.Status != "active" {
			return denied()
		}
		if e = s.reauthenticate(ctx, tx, u, password, code); e != nil {
			return e
		}
		if e = lockAccounts(ctx, tx, "wallet:"+u.ID); e != nil {
			return e
		}
		var balance, held Money
		if e = tx.QueryRowContext(ctx, `SELECT balance,reserved FROM accounts WHERE owner_id=$1`, u.ID).Scan(&balance, &held); e != nil {
			return e
		}
		var blocking bool
		if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM payments WHERE (owner_id=$1 OR recipient_id=$1) AND status NOT IN('succeeded','failed')) OR EXISTS(SELECT 1 FROM support_cases WHERE owner_id=$1 AND status<>'resolved')`, u.ID).Scan(&blocking); e != nil {
			return e
		}
		if balance != 0 || held != 0 || blocking {
			return conflict("withdraw or resolve remaining funds, payments and open cases before closing")
		}
		id := stringID("closure_")
		sealed, e := s.Config.Box.Seal(reason, "closure:"+id)
		if e != nil {
			return e
		}
		if e = exec(tx, ctx, `INSERT INTO closure_requests(id,owner_id,reason_enc,status) VALUES($1,$2,$3,'completed')`, id, u.ID, sealed); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE users SET status='closed',version=version+1 WHERE id=$1`, u.ID); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE sessions SET revoked=true WHERE user_id=$1`, u.ID); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE reminders SET status='cancelled',version=version+1 WHERE owner_id=$1 AND status IN('active','paused')`, u.ID); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE customer_controls SET discoverable=false,frozen=true WHERE owner_id=$1`, u.ID); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE payment_mandates SET status='cancelled',version=version+1 WHERE owner_id=$1 AND status IN('active','paused')`, u.ID); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE funding_accounts SET state='disabled' WHERE owner_id=$1`, u.ID); e != nil {
			return e
		}
		return s.audit(ctx, tx, u.ID, "account.closed", id, map[string]any{})
	})
}

// Export metadata in bounded pages. Credentials, challenge secrets, third-party
// recipients and restricted internal investigation records are never exported.
func (s *Service) PrivacyExport(ctx context.Context, p Principal, password, code string) (map[string]any, error) {
	if e := s.Rate(ctx, "privacy-export:"+p.User.ID, 3, time.Hour); e != nil {
		return nil, e
	}
	var out map[string]any
	e := s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if e = s.reauthenticate(ctx, tx, u, password, code); e != nil {
			return e
		}
		out = map[string]any{"profile": u, "generated_at": s.Now(), "schema_version": 1, "scope": "profile, preferences and customer controls; financial records are available through complete dated statements, documents through authenticated downloads", "financial_export_endpoint": "/v1/statements", "documents_endpoint": "/v1/uploads"}
		return s.audit(ctx, tx, u.ID, "privacy.exported", u.ID, map[string]any{})
	})
	if e != nil {
		return nil, e
	}
	preferences, e := s.Preferences(ctx, p.User.ID)
	if e != nil {
		return nil, e
	}
	controls, e := s.Controls(ctx, p)
	if e != nil {
		return nil, e
	}
	out["preferences"] = preferences
	out["controls"] = controls
	return out, nil
}
func (s *Service) EscalateCase(ctx context.Context, p Principal, id string) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		var state string
		if e = tx.QueryRowContext(ctx, `SELECT status FROM support_cases WHERE id=$1 AND owner_id=$2 FOR UPDATE`, id, u.ID).Scan(&state); e != nil {
			return isMissing(e)
		}
		if state == "resolved" {
			return conflict("reply to request reopening of this resolved case")
		}
		if state == "escalated" {
			return nil
		}
		if e = exec(tx, ctx, `UPDATE support_cases SET status='escalated',escalated_at=$2,updated_at=$2 WHERE id=$1`, id, s.Now()); e != nil {
			return e
		}
		if e = exec(tx, ctx, `INSERT INTO case_events(id,case_id,actor_id,event) VALUES($1,$2,$3,'customer_escalated')`, stringID("caseevt_"), id, u.ID); e != nil {
			return e
		}
		return s.audit(ctx, tx, u.ID, "case.escalated", id, map[string]any{})
	})
}
func (s *Service) CaseTimeline(ctx context.Context, p Principal, id string) ([]map[string]any, error) {
	var found string
	if e := s.DB.QueryRowContext(ctx, `SELECT id FROM support_cases WHERE id=$1 AND owner_id=$2`, id, p.User.ID).Scan(&found); e != nil {
		return nil, isMissing(e)
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT event,created_at FROM case_events WHERE case_id=$1 ORDER BY created_at,id`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var event string
		var at time.Time
		if e = rows.Scan(&event, &at); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"event": event, "at": at})
	}
	return out, rows.Err()
}
