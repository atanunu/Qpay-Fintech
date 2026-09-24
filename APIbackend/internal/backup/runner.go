package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type Process struct {
	Tool   string
	Args   []string
	Env    map[string]string
	Stdin  io.Reader
	Stdout io.Writer
	Dir    string
}
type Runner interface {
	Run(context.Context, Process) ([]byte, error)
}
type OSRunner struct{ Executables map[string]string }

var allowedEnvironment = map[string]bool{
	"PGHOST": true, "PGPORT": true, "PGDATABASE": true, "PGUSER": true, "PGPASSFILE": true, "PGSSLMODE": true, "PGSSLROOTCERT": true, "PGSSLCERT": true, "PGSSLKEY": true, "PGCONNECT_TIMEOUT": true,
	"AWS_ACCESS_KEY_ID": true, "AWS_SECRET_ACCESS_KEY": true, "AWS_SESSION_TOKEN": true, "AWS_REGION": true, "AWS_DEFAULT_REGION": true,
	"AZURE_ACCOUNT_NAME": true, "AZURE_ACCOUNT_KEY": true, "AZURE_ACCOUNT_SAS": true, "GOOGLE_APPLICATION_CREDENTIALS": true, "GOOGLE_PROJECT_ID": true,
	"B2_ACCOUNT_ID": true, "B2_ACCOUNT_KEY": true, "RCLONE_CONFIG": true, "REDISCLI_AUTH": true, "RESTIC_PASSWORD_FILE": true, "RESTIC_FROM_PASSWORD_FILE": true,
	"RESTIC_REPOSITORY": true, "RESTIC_FROM_REPOSITORY": true, "RESTIC_CACHE_DIR": true, "REDIS_HOST": true, "REDIS_PORT": true,
}

func ReadEnvironment(path string) (map[string]string, error) {
	out := map[string]string{}
	if path == "" {
		return out, nil
	}
	fi, e := os.Lstat(path)
	if e != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0077 != 0 || fi.Size() > 65536 {
		return nil, errors.New("credential profile requires a bounded owner-only JSON file")
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	if e = json.Unmarshal(raw, &out); e != nil {
		return nil, errors.New("credential profile must contain string environment values")
	}
	for k, v := range out {
		if !allowedEnvironment[k] || strings.ContainsRune(v, 0) || len(v) > 16384 {
			return nil, errors.New("unsupported credential environment variable")
		}
	}
	return out, nil
}

type limitBuffer struct {
	bytes.Buffer
	Limit int
}

func (b *limitBuffer) Write(v []byte) (int, error) {
	if b.Len()+len(v) > b.Limit {
		return 0, errors.New("bounded command output exceeded")
	}
	return b.Buffer.Write(v)
}
func (r OSRunner) Run(ctx context.Context, p Process) ([]byte, error) {
	tool, ok := r.Executables[p.Tool]
	if !ok || !filepath.IsAbs(tool) {
		return nil, errors.New("required backup tool is not provisioned")
	}
	fi, e := os.Stat(tool)
	if e != nil || !fi.Mode().IsRegular() {
		return nil, errors.New("backup binary unavailable")
	}
	cmd := exec.CommandContext(ctx, tool, p.Args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
	cmd.WaitDelay = 3 * time.Second
	paths := []string{filepath.Dir(tool)}
	for _, t := range r.Executables {
		paths = append(paths, filepath.Dir(t))
	}
	paths = append(paths, "/usr/bin", "/bin")
	cmd.Env = []string{"PATH=" + strings.Join(paths, ":"), "LANG=C.UTF-8", "LC_ALL=C.UTF-8"}
	// LD_LIBRARY_PATH is deliberately not inherited. Qualification harnesses may
	// wrap installed binaries, but untrusted profiles cannot preload process code.
	for k, v := range p.Env {
		if !allowedEnvironment[k] || strings.ContainsRune(v, 0) {
			return nil, errors.New("unsafe command environment")
		}
		if k != "REDIS_HOST" && k != "REDIS_PORT" {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	cmd.Dir = p.Dir
	cmd.Stdin = p.Stdin
	buf := &limitBuffer{Limit: 16 << 20}
	if p.Stdout != nil {
		cmd.Stdout = p.Stdout
	} else {
		cmd.Stdout = buf
	}
	cmd.Stderr = io.Discard
	if e = cmd.Run(); e != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("backup tool failed; inspect the protected host with an authorized operator")
	}
	return buf.Bytes(), nil
}
func resticProcess(profile DestinationProfile, command string, args ...string) (Process, error) {
	env, e := ReadEnvironment(profile.EnvironmentFile)
	if e != nil {
		return Process{}, e
	}
	env["RESTIC_REPOSITORY"] = profile.Repository
	env["RESTIC_PASSWORD_FILE"] = profile.PasswordFile
	return Process{Tool: "restic", Args: append([]string{"--json", command}, args...), Env: env}, nil
}
