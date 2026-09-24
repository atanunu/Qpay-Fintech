package backup

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
)

type Journal struct {
	Root *os.Root
	lock *os.File
	box  security.Box
}
type JournalState struct {
	Version    int                  `json:"version"`
	Serial     int64                `json:"serial"`
	Policy     Signed               `json:"policy"`
	Cursors    map[string]time.Time `json:"cursors"`
	LastSync   time.Time            `json:"last_sync"`
	LastResult time.Time            `json:"last_result"`
}
type JournalRun struct {
	Result   Result `json:"result"`
	Signed   Signed `json:"signed"`
	Policy   Signed `json:"policy"`
	Complete bool   `json:"complete"`
	Reported bool   `json:"reported"`
}

func OpenJournal(dir string, key []byte) (*Journal, error) {
	if len(key) != 32 || !filepath.IsAbs(dir) {
		return nil, errors.New("independent journal key and absolute path required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 || fi.Mode().Perm()&0077 != 0 {
		return nil, errors.New("journal must be an owner-only real directory")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	lock, err := root.OpenFile("agent.lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		root.Close()
		return nil, err
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		root.Close()
		return nil, errors.New("another backup agent already owns this journal")
	}
	return &Journal{Root: root, lock: lock, box: security.Box{Active: "journal", Keys: map[string][]byte{"journal": key}}}, nil
}
func (j *Journal) Close() error {
	syscall.Flock(int(j.lock.Fd()), syscall.LOCK_UN)
	j.lock.Close()
	return j.Root.Close()
}
func (j *Journal) Write(name string, v any) error {
	if !ID(name) {
		return errors.New("invalid journal record name")
	}
	sealed, err := j.box.Seal(string(Canonical(v)), "backup-journal:"+name)
	if err != nil {
		return err
	}
	temp := name + ".tmp"
	if fi, err := j.Root.Lstat(temp); err == nil && !fi.Mode().IsRegular() {
		return errors.New("unsafe journal temporary file")
	}
	f, err := j.Root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write([]byte(sealed)); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = j.Root.Rename(temp, name+".enc"); err != nil {
		return err
	}
	d, err := j.Root.Open(".")
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func (j *Journal) Read(name string, out any) error {
	if !ID(name) {
		return errors.New("invalid journal name")
	}
	f, err := j.Root.OpenFile(name+".enc", os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > 16<<20 {
		return errors.New("invalid journal record")
	}
	raw := make([]byte, fi.Size())
	if _, err = f.ReadAt(raw, 0); err != nil {
		return err
	}
	plain, err := j.box.Open(string(raw), "backup-journal:"+name)
	if err != nil {
		return errors.New("journal integrity or key mismatch")
	}
	return json.Unmarshal([]byte(plain), out)
}
func (j *Journal) State() (JournalState, error) {
	s := JournalState{Version: 1, Cursors: map[string]time.Time{}}
	err := j.Read("state", &s)
	if os.IsNotExist(err) {
		err = nil
	}
	if s.Version != 1 || s.Cursors == nil {
		return s, errors.New("journal schema unsupported")
	}
	return s, err
}
func (j *Journal) Runs() ([]JournalRun, error) {
	f, err := j.Root.Open(".")
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	f.Close()
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	out := []JournalRun{}
	for _, name := range names {
		if !strings.HasPrefix(name, "br_") || !strings.HasSuffix(name, ".enc") {
			continue
		}
		var r JournalRun
		if err = j.Read(strings.TrimSuffix(name, ".enc"), &r); err != nil {
			return nil, err
		}
		out = append(out, r)
		if len(out) > 100000 {
			return nil, errors.New("journal record capacity exceeded")
		}
	}
	return out, nil
}
func (j *Journal) RecoverInterrupted(key ed25519.PrivateKey, now time.Time) error {
	runs, err := j.Runs()
	if err != nil {
		return err
	}
	for _, entry := range runs {
		if entry.Complete {
			continue
		}
		r := entry.Result
		r.State = "unknown"
		r.ErrorCode = "interrupted"
		r.Copies = []Copy{}
		r.Files = []File{}
		r.FinishedAt = now
		if r.FinishedAt.Sub(r.StartedAt) > 24*time.Hour {
			r.FinishedAt = r.StartedAt.Add(24 * time.Hour)
		}
		entry.Result = r
		entry.Complete = true
		entry.Signed, err = Sign(r, key)
		if err != nil {
			return err
		}
		if err = j.Write(r.RunID, entry); err != nil {
			return err
		}
	}
	return nil
}
