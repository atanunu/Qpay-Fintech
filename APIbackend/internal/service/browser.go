package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
)

// BrowserCSRF is exposed only to approved browser origins with an unconsumed
// refresh cookie. It restores the CSRF value after a page reload without
// exposing access or refresh credentials to JavaScript.
func (s *Service) BrowserCSRF(ctx context.Context, refresh, audience string) (string, error) {
	if len(refresh) != 43 {
		return "", unauthorized()
	}
	var id, encrypted string
	e := s.DB.QueryRowContext(ctx, `SELECT s.id,s.csrf_enc FROM sessions s JOIN refresh_tokens t ON t.session_id=s.id JOIN users u ON u.id=s.user_id WHERE t.token_hash=$1 AND NOT t.consumed AND NOT s.revoked AND s.client='web' AND s.audience=$3 AND s.refresh_expires_at>$2 AND u.status<>'closed'`, security.Digest(refresh), s.Now(), audience).Scan(&id, &encrypted)
	if errors.Is(e, sql.ErrNoRows) {
		return "", unauthorized()
	}
	if e != nil {
		return "", e
	}
	return s.Config.Box.Open(encrypted, "csrf:"+id)
}
func (s *Service) Me(ctx context.Context, id string) (User, error) {
	return s.user(ctx, s.DB, id, false)
}
