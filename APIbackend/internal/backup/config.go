package backup

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"slices"
)

type RegisteredAgent struct {
	ID            string            `json:"id"`
	Environment   string            `json:"environment"`
	InstanceID    string            `json:"instance_id"`
	PublicKeyFile string            `json:"public_key_file"`
	Profiles      []Profile         `json:"profiles"`
	PublicKey     ed25519.PublicKey `json:"-"`
}
type ControlConfig struct {
	SigningKeyFile     string             `json:"signing_key_file"`
	LeaseSeconds       int                `json:"lease_seconds"`
	MaintenanceAllowed bool               `json:"maintenance_allowed,omitempty"`
	Agents             []RegisteredAgent  `json:"agents"`
	SigningKey         ed25519.PrivateKey `json:"-"`
}

func ReadKey(path string, private bool) ([]byte, error) {
	fi, e := os.Lstat(path)
	if e != nil || !fi.Mode().IsRegular() || fi.Size() > 1024 || private && fi.Mode().Perm()&0077 != 0 {
		return nil, errors.New("key requires a small regular file; private keys must be owner-only")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	b, e = base64.StdEncoding.DecodeString(string(bytesTrim(b)))
	if e != nil {
		return nil, errors.New("key file must contain base64")
	}
	if len(b) != 32 {
		return nil, errors.New("backup keys must decode to exactly 32 bytes")
	}
	return b, nil
}
func bytesTrim(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}
func LoadControl(path string) (*ControlConfig, error) {
	if path == "" {
		return nil, nil
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var c ControlConfig
	if e = Decode(b, &c); e != nil {
		return nil, e
	}
	k, e := ReadKey(c.SigningKeyFile, true)
	if e != nil || len(k) != ed25519.SeedSize {
		return nil, errors.New("backup policy seed must be a protected independent 32-byte key")
	}
	c.SigningKey = ed25519.NewKeyFromSeed(k)
	if c.LeaseSeconds < 300 || c.LeaseSeconds > 86400 {
		return nil, errors.New("backup policy lease must be 300–86400 seconds")
	}
	seen := map[string]bool{}
	for i, a := range c.Agents {
		if !ID(a.ID) || !ID(a.InstanceID) || seen[a.ID] || !slices.Contains([]string{"local", "staging", "production"}, a.Environment) {
			return nil, errors.New("invalid or duplicate backup agent")
		}
		seen[a.ID] = true
		k, e := ReadKey(a.PublicKeyFile, false)
		if e != nil || len(k) != ed25519.PublicKeySize {
			return nil, errors.New("agent public key is invalid")
		}
		c.Agents[i].PublicKey = ed25519.PublicKey(k)
		profiles := map[string]bool{}
		for _, p := range a.Profiles {
			if !ID(p.ID) || profiles[p.Kind+":"+p.ID] || !slices.Contains([]string{"source", "destination", "restore"}, p.Kind) || !Text(p.Label, 140) {
				return nil, errors.New("invalid profile catalogue")
			}
			profiles[p.Kind+":"+p.ID] = true
		}
	}
	return &c, nil
}
func (c *ControlConfig) Agent(id string) (RegisteredAgent, bool) {
	if c != nil {
		for _, a := range c.Agents {
			if a.ID == id {
				return a, true
			}
		}
	}
	return RegisteredAgent{}, false
}
func (c *ControlConfig) Profile(agent, kind, id string) (Profile, bool) {
	a, ok := c.Agent(agent)
	if ok {
		for _, p := range a.Profiles {
			if p.ID == id && p.Kind == kind {
				return p, true
			}
		}
	}
	return Profile{}, false
}
func (c *ControlConfig) Keys() map[string]ed25519.PublicKey {
	k := map[string]ed25519.PublicKey{}
	if c != nil {
		for _, a := range c.Agents {
			k[a.ID] = a.PublicKey
		}
	}
	return k
}

// Local executable profiles are deployment-owned. No request can set these paths,
// repository URLs, executable arguments or environment files.
type SourceProfile struct {
	Type             string `json:"type"`
	Root             string `json:"root,omitempty"`
	EnvironmentFile  string `json:"environment_file,omitempty"`
	ObjectRoot       string `json:"object_root,omitempty"`
	ObjectProfile    string `json:"object_profile,omitempty"`
	PgBackRestConfig string `json:"pgbackrest_config,omitempty"`
	Stanza           string `json:"stanza,omitempty"`
	MongoConfig      string `json:"mongo_config,omitempty"`
	RedisTLS         bool   `json:"redis_tls,omitempty"`
	RedisCACert      string `json:"redis_ca_cert,omitempty"`
	MongoReplicaSet  bool   `json:"mongo_replica_set,omitempty"`
}
type DestinationProfile struct {
	Provider                   string `json:"provider"`
	Repository                 string `json:"repository"`
	PasswordFile               string `json:"password_file"`
	EnvironmentFile            string `json:"environment_file,omitempty"`
	PgRepository               int    `json:"pg_repository,omitempty"`
	PgBackRestConfig           string `json:"pgbackrest_config,omitempty"`
	Stanza                     string `json:"stanza,omitempty"`
	FailureDomain              string `json:"failure_domain"`
	OffHost                    bool   `json:"off_host"`
	Region                     string `json:"region"`
	MaintenanceEnvironmentFile string `json:"maintenance_environment_file,omitempty"`
}
type ObjectSourceProfile struct {
	Endpoint          string `json:"endpoint"`
	Region            string `json:"region"`
	Bucket            string `json:"bucket"`
	Prefix            string `json:"prefix"`
	EnvironmentFile   string `json:"environment_file"`
	RequireVersioning bool   `json:"require_versioning"`
}
type AgentConfig struct {
	ID                       string                         `json:"id"`
	InstanceID               string                         `json:"instance_id"`
	Environment              string                         `json:"environment"`
	APIOrigin                string                         `json:"api_origin"`
	PrivateKeyFile           string                         `json:"private_key_file"`
	PolicyPublicKeyFile      string                         `json:"policy_public_key_file"`
	StateKeyFile             string                         `json:"state_key_file"`
	StateDir                 string                         `json:"state_dir"`
	ScratchDir               string                         `json:"scratch_dir"`
	Stage                    DestinationProfile             `json:"stage"`
	ObjectSources            map[string]ObjectSourceProfile `json:"object_sources,omitempty"`
	Sources                  map[string]SourceProfile       `json:"sources"`
	Destinations             map[string]DestinationProfile  `json:"destinations"`
	RecoveryCustodians       map[string]string              `json:"recovery_custodians,omitempty"`
	RestoreRoots             map[string]string              `json:"restore_roots"`
	Executables              map[string]string              `json:"executables"`
	EncryptedScratchAccepted bool                           `json:"encrypted_scratch_accepted"`
	MaintenanceEnabled       bool                           `json:"maintenance_enabled"`
	Catalogue                []Profile                      `json:"catalogue"`
	TimeoutSeconds           int                            `json:"timeout_seconds"`
	MaxStageBytes            int64                          `json:"max_stage_bytes"`
}

func (c AgentConfig) Validate() error {
	if !ID(c.ID) || !ID(c.InstanceID) || !slices.Contains([]string{"local", "staging", "production"}, c.Environment) || c.TimeoutSeconds < 60 || c.TimeoutSeconds > 86400 || c.MaxStageBytes < 1<<20 {
		return errors.New("agent identity, timeout and staging budget required")
	}
	if c.Environment != "local" && !c.EncryptedScratchAccepted {
		return errors.New("deployment owner must qualify encrypted scratch storage before non-local backup")
	}
	for _, p := range []string{c.StateDir, c.ScratchDir, c.Stage.Repository} {
		if !filepath.IsAbs(p) || filepath.Clean(p) == "/" {
			return errors.New("dedicated absolute state, scratch and encrypted staging paths required")
		}
	}
	if c.Stage.Provider != "local" {
		return errors.New("encrypted staging repository must be local")
	}
	for _, p := range c.RestoreRoots {
		if !filepath.IsAbs(p) || filepath.Clean(p) == "/" {
			return errors.New("isolated restore root must be absolute and non-root")
		}
		for _, s := range c.Sources {
			if within(s.Root, p) || within(p, s.Root) || within(s.ObjectRoot, p) || within(p, s.ObjectRoot) {
				return errors.New("restore roots must not overlap source roots")
			}
		}
	}
	for name, p := range c.Sources {
		if !ID(name) {
			return errors.New("invalid source profile name")
		}
		if p.Root != "" {
			for _, forbidden := range []string{c.StateDir, c.ScratchDir, c.Stage.Repository} {
				if within(p.Root, forbidden) || within(forbidden, p.Root) {
					return errors.New("source cannot include backup output/state")
				}
			}
		}
	}
	for name, p := range c.Destinations {
		if !ID(name) || !Text(p.FailureDomain, 100) {
			return errors.New("destination profile and failure domain required")
		}
		if p.Provider != "pgbackrest" && (p.PasswordFile == "" || p.Repository == "") {
			return errors.New("encrypted repository and password-file reference required")
		}
	}
	for name, p := range c.Executables {
		if !slices.Contains([]string{"restic", "pg_dump", "pg_dumpall", "pgbackrest", "mongodump", "redis-cli", "rclone", "pg_restore"}, name) || !filepath.IsAbs(p) {
			return errors.New("executable allowlist accepts only explicit supported binary paths")
		}
	}
	return nil
}
func within(root, path string) bool {
	if root == "" || path == "" {
		return false
	}
	r, e := filepath.Rel(root, path)
	return e == nil && (r == "." || r != ".." && !stringsHasParent(r))
}
func stringsHasParent(r string) bool { return len(r) > 3 && r[:3] == ".."+string(filepath.Separator) }
