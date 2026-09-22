// Package app wires separately deployable processes from the same domain code.
package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/httpapi"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/service"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/upstream"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func required(name string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}
func truth(name string) bool { return os.Getenv(name) == "true" }
func decodeKey(v string) ([]byte, error) {
	raw, e := base64.StdEncoding.DecodeString(v)
	if e != nil {
		raw, e = base64.RawStdEncoding.DecodeString(v)
	}
	if e != nil || len(raw) != 32 {
		return nil, errors.New("key must be base64 encoded 32 bytes")
	}
	return raw, nil
}
func Load(ctx context.Context) (*service.Service, httpapi.Config, error) {
	var c service.Config
	environment, e := required("QPF_ENV")
	if e != nil {
		return nil, httpapi.Config{}, e
	}
	if environment != "local" && environment != "staging" && environment != "production" {
		return nil, httpapi.Config{}, errors.New("QPF_ENV must be local, staging or production")
	}
	c.Environment = environment
	urlText, e := required("DATABASE_URL")
	if e != nil {
		return nil, httpapi.Config{}, e
	}
	u, e := url.Parse(urlText)
	if e != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" {
		return nil, httpapi.Config{}, errors.New("valid PostgreSQL DATABASE_URL required")
	}
	if environment != "local" && u.Query().Get("sslmode") != "verify-full" {
		return nil, httpapi.Config{}, errors.New("non-local database requires sslmode=verify-full")
	}
	c.Pepper, e = decodeKey(os.Getenv("AUTH_PEPPER"))
	if e != nil {
		return nil, httpapi.Config{}, fmt.Errorf("AUTH_PEPPER: %w", e)
	}
	keys := map[string][]byte{}
	for _, entry := range strings.Split(os.Getenv("DATA_KEYS"), ",") {
		id, value, ok := strings.Cut(entry, ":")
		if !ok || id == "" || strings.Contains(id, ".") {
			return nil, httpapi.Config{}, errors.New("DATA_KEYS requires key-id:base64 entries")
		}
		if _, exists := keys[id]; exists {
			return nil, httpapi.Config{}, errors.New("duplicate data key id")
		}
		key, e := decodeKey(value)
		if e != nil {
			return nil, httpapi.Config{}, e
		}
		keys[id] = key
	}
	c.Box = security.Box{Active: os.Getenv("DATA_KEY_ID"), Keys: keys}
	if _, e = c.Box.Seal("config-check", "startup"); e != nil {
		return nil, httpapi.Config{}, e
	}
	origins := strings.Split(os.Getenv("WEB_ORIGINS"), ",")
	for _, origin := range origins {
		parsed, e := url.Parse(origin)
		if e != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" || (parsed.Scheme != "https" && (environment != "local" || parsed.Scheme != "http")) {
			return nil, httpapi.Config{}, errors.New("WEB_ORIGINS requires exact HTTPS origins (local HTTP permitted)")
		}
	}
	c.Origins = origins
	mode := os.Getenv("EXECUTION_MODE")
	if mode == "" {
		mode = "off"
	}
	c.PolicyAccepted = truth("ACCOUNT_POLICY_ACCEPTED")
	c.NotificationMode = os.Getenv("NOTIFICATION_MODE")
	if c.NotificationMode == "" {
		c.NotificationMode = "off"
	}
	if c.NotificationMode != "off" && c.NotificationMode != "local" && c.NotificationMode != "novu" {
		return nil, httpapi.Config{}, errors.New("invalid NOTIFICATION_MODE")
	}
	if c.NotificationMode == "local" && environment != "local" {
		return nil, httpapi.Config{}, errors.New("local notification capture prohibited outside local environment")
	}
	if key := os.Getenv("NOTIFICATION_POLICY_KEY"); key != "" {
		c.PolicyKey, e = decodeKey(key)
		if e != nil {
			return nil, httpapi.Config{}, e
		}
	}
	if c.NotificationMode == "novu" {
		c.NovuURL = os.Getenv("NOVU_API_URL")
		v, e := url.Parse(c.NovuURL)
		if e != nil || v.Scheme != "https" || v.Hostname() == "" || v.User != nil || v.RawQuery != "" || v.Fragment != "" || strings.Trim(v.Path, "/") != "" {
			return nil, httpapi.Config{}, errors.New("NOVU_API_URL requires a self-hosted HTTPS origin")
		}
		c.NovuKey, e = required("NOVU_SECRET_KEY")
		if e != nil {
			return nil, httpapi.Config{}, e
		}
		if len(c.PolicyKey) != 32 {
			return nil, httpapi.Config{}, errors.New("signed notification policy key required")
		}
		c.NovuAllowlist = map[string]bool{}
		for _, id := range strings.Split(os.Getenv("NOVU_WORKFLOW_ALLOWLIST"), ",") {
			if id == "" {
				continue
			}
			key := strings.TrimSuffix(strings.TrimPrefix(id, "qpf-email-"), "-v1")
			meta, exists := service.NotificationMetadata[key]
			if !exists || meta.Scope != "core" || id != "qpf-email-"+key+"-v1" {
				return nil, httpapi.Config{}, errors.New("unknown or unapproved workflow allowlist entry")
			}
			c.NovuAllowlist[id] = true
		}
	}
	db, e := sql.Open("pgx", urlText)
	if e != nil {
		return nil, httpapi.Config{}, e
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if e = db.PingContext(ctx); e != nil {
		db.Close()
		return nil, httpapi.Config{}, errors.New("database connection failed")
	}
	fail := func(e error) (*service.Service, httpapi.Config, error) { db.Close(); return nil, httpapi.Config{}, e }
	switch mode {
	case "off":
	case "local":
		if environment != "local" {
			return fail(errors.New("synthetic execution prohibited outside local environment"))
		}
		c.Gateway = &upstream.Local{DB: db}
		c.ExternalEnabled = true
	case "qpay":
		if !truth("QPAY_CONTRACT_ACCEPTED") {
			return fail(errors.New("explicit QPay contract qualification required"))
		}
		upEnv := os.Getenv("QPAY_ENVIRONMENT")
		if (environment == "production") != (upEnv == "production") {
			return fail(errors.New("QPay environment does not match deployment"))
		}
		gateway, e := upstream.NewQPay(os.Getenv("QPAY_URL"), os.Getenv("QPAY_APPLICATION_ID"), os.Getenv("QPAY_API_KEY"), os.Getenv("QPAY_API_SECRET"), upEnv, service.Destination{AccountNumber: os.Getenv("QPAY_SENDER_ACCOUNT"), AccountName: os.Getenv("QPAY_SENDER_NAME"), BankCode: os.Getenv("QPAY_SENDER_BANK")})
		if e != nil {
			return fail(e)
		}
		c.Gateway = gateway
		c.ExternalEnabled = true
	default:
		return fail(errors.New("EXECUTION_MODE must be off, local or qpay"))
	}
	return service.New(db, c), httpapi.Config{Environment: environment, Origins: origins, Logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)), PolicyKey: c.PolicyKey}, nil
}
func Main(kind string) {
	if e := Run(kind); e != nil {
		slog.Error("process_stopped", "process", kind, "error", e.Error())
		os.Exit(1)
	}
}
func Run(kind string) error {
	if kind == "qpfctl" && len(os.Args) > 1 && os.Args[1] == "init-local-env" {
		return initLocal()
	}
	if kind == "contracts" {
		return contracts()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	s, c, e := Load(startup)
	if e != nil {
		return e
	}
	defer s.DB.Close()
	if kind == "migrate" {
		return service.Migrate(ctx, s.DB)
	}
	if e = s.Ping(ctx); e != nil {
		return errors.New("apply reviewed migrations before starting this process")
	}
	if kind == "qpfctl" {
		return control(ctx, s)
	}
	if kind == "api" {
		address := os.Getenv("HTTP_ADDR")
		if address == "" {
			address = ":8080"
		}
		server := &http.Server{Addr: address, Handler: httpapi.New(s, c), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 90 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32768}
		done := make(chan error, 1)
		go func() { done <- server.ListenAndServe() }()
		c.Logger.Info("api_started", "environment", s.Config.Environment)
		select {
		case err := <-done:
			if !errors.Is(err, http.ErrServerClosed) {
				return err
			}
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			return server.Shutdown(shutdown)
		}
		return nil
	}
	if kind != "worker" && kind != "scheduler" {
		return errors.New("unknown process")
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	last := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if kind == "scheduler" {
				if time.Since(last) < time.Minute {
					continue
				}
				last = time.Now()
				if e = s.Scheduler(ctx); e != nil {
					c.Logger.Error("scheduler_failed", "error_type", fmt.Sprintf("%T", e))
				}
				continue
			}
			for i := 0; i < 10; i++ {
				worked, e := s.WorkPayment(ctx)
				if e != nil {
					c.Logger.Error("payment_worker_failed", "error_type", fmt.Sprintf("%T", e))
				}
				sent, ne := s.WorkNotification(ctx)
				if ne != nil {
					c.Logger.Error("notification_worker_failed", "error_type", fmt.Sprintf("%T", ne))
				}
				if !worked && !sent {
					break
				}
			}
		}
	}
}
func readSecret(path string) (string, error) {
	if path == "" {
		return "", errors.New("secret file path required")
	}
	stat, e := os.Stat(path)
	if e != nil {
		return "", e
	}
	if stat.Mode().Perm()&0077 != 0 {
		return "", errors.New("secret file must be owner-only (chmod 600)")
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	return strings.TrimRight(string(raw), "\r\n"), nil
}
func control(ctx context.Context, s *service.Service) error {
	if len(os.Args) < 2 {
		return errors.New("commands: bootstrap-admin, seed-local, show-local-code; pass values through QPF_CTL_* environment and owner-only secret files")
	}
	command := os.Args[1]
	if command == "show-local-code" {
		value, e := s.LocalChallenge(ctx, os.Getenv("QPF_CTL_EMAIL"))
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(value)
	}
	password, e := readSecret(os.Getenv("QPF_CTL_PASSWORD_FILE"))
	if e != nil {
		return e
	}
	var id string
	switch command {
	case "bootstrap-admin":
		id, e = s.BootstrapStaff(ctx, os.Getenv("QPF_CTL_EMAIL"), os.Getenv("QPF_CTL_NAME"), password)
	case "seed-local":
		pin, err := readSecret(os.Getenv("QPF_CTL_PIN_FILE"))
		if err != nil {
			return err
		}
		id, e = s.SeedLocal(ctx, os.Getenv("QPF_CTL_EMAIL"), os.Getenv("QPF_CTL_NAME"), password, pin)
	default:
		return errors.New("unknown control command")
	}
	if e != nil {
		return e
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]string{"id": id, "environment": s.Config.Environment})
}
func initLocal() error {
	file := ".env.local"
	if len(os.Args) > 2 {
		file = os.Args[2]
	}
	f, e := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	key := func() string {
		raw := make([]byte, 32)
		if _, e := rand.Read(raw); e != nil {
			panic("random source unavailable")
		}
		return base64.StdEncoding.EncodeToString(raw)
	}
	password := security.Random("", 24)
	_, e = fmt.Fprintf(f, "# LOCAL SYNTHETIC DEVELOPMENT ONLY\nQPF_ENV=local\nPOSTGRES_PASSWORD=%s\nDATABASE_URL=postgres://qpf:%s@postgres:5432/qpf?sslmode=disable\nAUTH_PEPPER=%s\nDATA_KEY_ID=v1\nDATA_KEYS=v1:%s\nNOTIFICATION_POLICY_KEY=%s\nWEB_ORIGINS=http://localhost:5173,http://localhost:5174\nEXECUTION_MODE=local\nNOTIFICATION_MODE=local\nHTTP_ADDR=:8080\n", password, password, key(), key(), key())
	return e
}
func contracts() error {
	h := httpapi.New(&service.Service{}, httpapi.Config{Environment: "production"})
	raw, e := json.MarshalIndent(h.OpenAPI(), "", "  ")
	if e != nil {
		return e
	}
	dir := "contracts"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if e = os.MkdirAll(dir, 0755); e != nil {
		return e
	}
	if e = os.WriteFile(dir+"/openapi.json", append(raw, '\n'), 0644); e != nil {
		return e
	}
	var b strings.Builder
	b.WriteString("// Generated from registered Go routes. Do not edit.\nexport type APIPath =\n")
	for i, r := range h.Routes {
		if i > 0 {
			b.WriteString(" |\n")
		}
		b.WriteString(strconv.Quote(r.Method + " " + r.Path))
	}
	b.WriteString(";\n")
	return os.WriteFile(dir+"/paths.ts", []byte(b.String()), 0644)
}
