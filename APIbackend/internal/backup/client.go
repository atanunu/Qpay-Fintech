package backup

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	Origin, AgentID string
	Private         ed25519.PrivateKey
	HTTP            *http.Client
	Now             func() time.Time
}

func NewClient(c AgentConfig, key ed25519.PrivateKey) (*Client, error) {
	u, err := url.Parse(c.APIOrigin)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || u.Scheme != "https" && (c.Environment != "local" || u.Scheme != "http" || !strings.EqualFold(u.Hostname(), "localhost") && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") {
		return nil, errors.New("agent control origin must be exact HTTPS; local loopback HTTP is the only exception")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	transport.Proxy = nil
	return &Client{Origin: c.APIOrigin, AgentID: c.ID, Private: key, Now: time.Now, HTTP: &http.Client{Timeout: 60 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func nonce() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return "bn_" + hex.EncodeToString(b)
}
func (c *Client) Request(ctx context.Context, method, path string, in, out any) error {
	if !strings.HasPrefix(path, "/v1/backup-agent/") || strings.ContainsAny(path, "?#") {
		return errors.New("invalid agent route")
	}
	raw := []byte{}
	if in != nil {
		raw = Canonical(in)
	}
	auth, err := EncodeAuth(Authentication{c.AgentID, method, path, c.Now(), nonce(), Digest(raw)}, c.Private)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Origin+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("X-Backup-Agent", auth)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return errors.New("backup control API unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("backup control API rejected request; inspect authorized run metadata")
	}
	if resp.StatusCode == 204 {
		return nil
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20+1))
	if err != nil || len(b) > 8<<20 {
		return errors.New("invalid backup control response")
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(b, &envelope) != nil || len(envelope.Data) == 0 {
		return errors.New("missing backup response data")
	}
	if out != nil {
		return json.Unmarshal(envelope.Data, out)
	}
	return nil
}
