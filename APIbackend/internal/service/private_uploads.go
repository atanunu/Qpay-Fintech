package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
)

type UploadStore interface {
	Put(context.Context, string, []byte) error
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}
type UploadScanner interface {
	Scan(context.Context, []byte) (string, error)
}
type Upload struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Purpose   string    `json:"purpose"`
	CaseID    string    `json:"case_id"`
	Mime      string    `json:"mime"`
	Size      int64     `json:"size"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
}

func CheckUpload(name string, data []byte) (string, error) {
	if !safeText(name, 150) || filepath.Base(name) != name || strings.ContainsAny(name, "/\\") || len(data) == 0 || len(data) > 5<<20 {
		return "", Invalid("choose a JPEG, PNG or PDF up to 5 MiB with a plain filename")
	}
	mime := http.DetectContentType(data)
	extension := strings.ToLower(filepath.Ext(name))
	if mime == "image/png" && extension != ".png" || mime == "image/jpeg" && extension != ".jpg" && extension != ".jpeg" || mime == "application/pdf" && extension != ".pdf" {
		return "", Invalid("file content does not match its extension")
	}
	if mime != "image/png" && mime != "image/jpeg" && mime != "application/pdf" {
		return "", Invalid("only JPEG, PNG and PDF documents are accepted")
	}
	if strings.HasPrefix(mime, "image/") {
		config, _, e := image.DecodeConfig(bytes.NewReader(data))
		if e != nil || config.Width < 1 || config.Height < 1 || config.Width > 8000 || config.Height > 8000 || int64(config.Width)*int64(config.Height) > 20000000 {
			return "", Invalid("image dimensions are invalid or exceed 20 megapixels")
		}
	}
	if mime == "application/pdf" && (bytes.Contains(data, []byte("/Encrypt")) || !bytes.Contains(data, []byte("%%EOF"))) {
		return "", Invalid("encrypted or incomplete PDF files are not accepted")
	}
	return mime, nil
}
func (s *Service) UploadDocument(ctx context.Context, p Principal, purpose, caseID, name string, data []byte) (Upload, error) {
	var out Upload
	if e := p.Customer(); e != nil {
		return out, e
	}
	if s.Config.UploadStore == nil || s.Config.UploadScanner == nil {
		return out, &Fault{503, "uploads_unavailable", "private document storage and scanning are not configured"}
	}
	if purpose != "kyc" && purpose != "support" {
		return out, Invalid("invalid upload purpose")
	}
	if purpose == "support" && !validID(caseID) {
		return out, Invalid("a support case is required")
	}
	if purpose == "kyc" && caseID != "" {
		return out, Invalid("KYC upload must not reference a support case")
	}
	mime, e := CheckUpload(name, data)
	if e != nil {
		return out, e
	}
	if e = s.Rate(ctx, "uploads:"+p.User.ID, 20, time.Hour); e != nil {
		return out, e
	}
	scanCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	state, e := s.Config.UploadScanner.Scan(scanCtx, data)
	if e != nil {
		return out, &Fault{503, "scan_unavailable", "document scanning is unavailable; no document was accepted"}
	}
	if state != "clean" && (state != "local_unscanned" || s.Config.Environment != "local") {
		return out, &Fault{422, "document_rejected", "the document did not pass content checks"}
	}
	id := stringID("upload_")
	encrypted, e := s.Config.Box.Seal(base64.StdEncoding.EncodeToString(data), "upload:"+p.User.ID+":"+id)
	if e != nil {
		return out, e
	}
	fileName, e := s.Config.Box.Seal(name, "upload-name:"+id)
	if e != nil {
		return out, e
	}
	// Verify the session, purpose and quota before external storage, then again before publication.
	e = s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if !u.Verified {
			return denied()
		}
		var count int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM private_uploads WHERE owner_id=$1 AND state<>'deleted'`, u.ID).Scan(&count); e != nil {
			return e
		}
		if count >= 100 {
			return Invalid("document quota reached")
		}
		if purpose == "support" {
			var owner string
			if e = tx.QueryRowContext(ctx, `SELECT owner_id FROM support_cases WHERE id=$1 AND owner_id=$2`, caseID, u.ID).Scan(&owner); e != nil {
				return isMissing(e)
			}
		}
		return nil
	})
	if e != nil {
		return out, e
	}
	if e = s.Config.UploadStore.Put(ctx, id, []byte(encrypted)); e != nil {
		return out, &Fault{503, "storage_unavailable", "private storage did not accept the document"}
	}
	e = s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		var count int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM private_uploads WHERE owner_id=$1 AND state<>'deleted'`, u.ID).Scan(&count); e != nil {
			return e
		}
		if count >= 100 {
			return Invalid("document quota reached")
		}
		var caseValue any
		if caseID != "" {
			caseValue = caseID
		}
		if e = exec(tx, ctx, `INSERT INTO private_uploads(id,owner_id,purpose,case_id,object_key,mime,size,digest,name_enc,state) VALUES($1,$2,$3,$4,$1,$5,$6,$7,$8,$9)`, id, u.ID, purpose, caseValue, mime, len(data), security.Digest(string(data)), fileName, state); e != nil {
			return e
		}
		return s.audit(ctx, tx, u.ID, "document.stored", id, map[string]any{"purpose": purpose, "state": state})
	})
	if e != nil {
		cleanup, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = s.Config.UploadStore.Delete(cleanup, id)
		return out, e
	}
	out = Upload{ID: id, Name: name, Purpose: purpose, CaseID: caseID, Mime: mime, Size: int64(len(data)), State: state, CreatedAt: s.Now()}
	return out, nil
}
func (s *Service) Uploads(ctx context.Context, p Principal, purpose, caseID string) ([]Upload, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id,name_enc,purpose,coalesce(case_id,''),mime,size,state,created_at FROM private_uploads WHERE owner_id=$1 AND state<>'deleted' AND ($2='' OR purpose=$2) AND ($3='' OR case_id=$3) ORDER BY created_at DESC LIMIT 100`, p.User.ID, purpose, caseID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Upload{}
	for rows.Next() {
		var v Upload
		var sealed string
		if e = rows.Scan(&v.ID, &sealed, &v.Purpose, &v.CaseID, &v.Mime, &v.Size, &v.State, &v.CreatedAt); e != nil {
			return nil, e
		}
		v.Name, e = s.Config.Box.Open(sealed, "upload-name:"+v.ID)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) DownloadDocument(ctx context.Context, p Principal, id string) (Upload, []byte, error) {
	var v Upload
	var owner, key, name, digest string
	e := s.DB.QueryRowContext(ctx, `SELECT owner_id,object_key,name_enc,digest,id,purpose,coalesce(case_id,''),mime,size,state,created_at FROM private_uploads WHERE id=$1 AND state IN('clean','local_unscanned')`, id).Scan(&owner, &key, &name, &digest, &v.ID, &v.Purpose, &v.CaseID, &v.Mime, &v.Size, &v.State, &v.CreatedAt)
	if e != nil {
		return v, nil, isMissing(e)
	}
	if p.Audience == "customer" {
		if owner != p.User.ID {
			return v, nil, missing()
		}
	} else {
		roles := []string{"admin", "compliance"}
		if v.Purpose == "support" {
			roles = []string{"admin", "support"}
		}
		if e = requireRole(p, roles...); e != nil {
			return v, nil, e
		}
	}
	if v.State == "local_unscanned" && s.Config.Environment != "local" {
		return v, nil, denied()
	}
	if s.Config.UploadStore == nil {
		return v, nil, unavailable()
	}
	raw, e := s.Config.UploadStore.Get(ctx, key)
	if e != nil {
		return v, nil, unavailable()
	}
	plain, e := s.Config.Box.Open(string(raw), "upload:"+owner+":"+id)
	if e != nil {
		return v, nil, e
	}
	data, e := base64.StdEncoding.DecodeString(plain)
	if e != nil || int64(len(data)) != v.Size || security.Digest(string(data)) != digest {
		return v, nil, errors.New("private document integrity mismatch")
	}
	v.Name, e = s.Config.Box.Open(name, "upload-name:"+id)
	if e != nil {
		return v, nil, e
	}
	e = s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		p.User = u
		if p.Audience != "customer" {
			roles := []string{"admin", "compliance"}
			if v.Purpose == "support" {
				roles = []string{"admin", "support"}
			}
			if e = requireRole(p, roles...); e != nil {
				return e
			}
		}
		var usable bool
		if e = tx.QueryRowContext(ctx, `SELECT state IN('clean','local_unscanned') FROM private_uploads WHERE id=$1 FOR SHARE`, id).Scan(&usable); e != nil {
			return e
		}
		if !usable {
			return missing()
		}
		return s.audit(ctx, tx, p.User.ID, "document.downloaded", id, map[string]any{"purpose": v.Purpose})
	})
	return v, data, e
}
func (s *Service) DeleteUpload(ctx context.Context, p Principal, id string) error {
	e := s.transact(ctx, func(tx *sql.Tx) error {
		if _, e := s.activePrincipal(ctx, tx, p); e != nil {
			return e
		}
		var connected bool
		e := tx.QueryRowContext(ctx, `SELECT purpose='support' OR EXISTS(SELECT 1 FROM kyc_documents WHERE upload_id=$1) FROM private_uploads WHERE id=$1 AND owner_id=$2 AND state<>'deleted' FOR UPDATE`, id, p.User.ID).Scan(&connected)
		if e != nil {
			return isMissing(e)
		}
		if connected {
			return conflict("submitted evidence is retained with its case; use the privacy/support process")
		}
		if e = exec(tx, ctx, `UPDATE private_uploads SET state='deleted' WHERE id=$1`, id); e != nil {
			return e
		}
		return s.audit(ctx, tx, p.User.ID, "document.deleted", id, map[string]any{})
	})
	if e != nil {
		return e
	}
	if s.Config.UploadStore != nil {
		return s.Config.UploadStore.Delete(ctx, id)
	}
	return nil
}
