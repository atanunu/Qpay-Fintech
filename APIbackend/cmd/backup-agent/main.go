// backup-agent executes no financial or notification work. Run on an isolated,
// minimally privileged host with provisioned keys, profiles and read-only sources.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/backup"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	flags := flag.NewFlagSet("backup-agent", flag.ContinueOnError)
	config := flags.String("config", "", "owner-only agent JSON configuration")
	profile := flags.String("profile", "stage", "repository profile for explicit init")
	output := flags.String("output", "", "new key directory or protected output file")
	bundleFile := flags.String("bundle", "", "owner-only offline recovery bundle")
	custodian := flags.String("custodian", "", "enrolled independent recovery custodian")
	custodianKey := flags.String("custodian-key", "", "protected offline custodian signing seed")
	runID := flags.String("run", "", "original recovery run reference")
	copyID := flags.String("copy", "", "exact copy reference")
	target := flags.String("target", "", "isolated registered recovery target")
	operation := flags.String("operation", "restore", "restore or encrypted export")
	healthFile := flags.String("health-file", "", "signed non-secret agent health file")
	publicFile := flags.String("public-key", "", "agent public key for independent watchdog")
	heartbeatAge := flags.Duration("heartbeat-max-age", 5*time.Minute, "maximum heartbeat age")
	captureAge := flags.Duration("capture-max-age", 26*time.Hour, "maximum complete capture age")
	ack := flags.Bool("ack-isolated", false, "acknowledge isolated target and disabled financial/notification execution")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: backup-agent -config /run/secrets/agent.json [-profile name] check|init|once|watch|status; or -output NEW_DIRECTORY keygen")
	}
	command := flags.Arg(0)
	if command == "watchdog" {
		public, err := backup.ReadKey(*publicFile, false)
		if err != nil {
			return err
		}
		h, err := backup.CheckHealth(*healthFile, ed25519.PublicKey(public), time.Now(), *heartbeatAge, *captureAge)
		message := "healthy"
		if err != nil {
			message = err.Error()
		}
		if e := json.NewEncoder(os.Stdout).Encode(map[string]any{"health": h, "status": message}); e != nil {
			return e
		}
		return err
	}
	if command == "offline-approve" {
		if !*ack {
			return errors.New("inspect the offline request and acknowledge the isolated recovery boundary")
		}
		b, err := backup.ReadOfflineBundle(*bundleFile)
		if err != nil {
			return err
		}
		seed, err := backup.ReadKey(*custodianKey, true)
		if err != nil {
			return err
		}
		b, err = backup.ApproveOffline(b, *custodian, ed25519.NewKeyFromSeed(seed))
		if err != nil {
			return err
		}
		return backup.WriteNewProtected(*output, b)
	}
	if command == "keygen" {
		if *output == "" || !filepath.IsAbs(*output) {
			return errors.New("a new absolute key directory is required")
		}
		return keygen(*output)
	}
	fi, err := os.Lstat(*config)
	if err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0077 != 0 || fi.Size() > 1<<20 {
		return errors.New("agent configuration requires an owner-only regular file")
	}
	raw, err := os.ReadFile(*config)
	if err != nil {
		return err
	}
	var c backup.AgentConfig
	if err = backup.Decode(raw, &c); err != nil {
		return err
	}
	if err = c.Validate(); err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if command == "check" {
		fmt.Println("Agent configuration syntax accepted; no source, account or repository has been tested.")
		return nil
	}
	if command == "init" {
		e, err := backup.NewEngine(c)
		if err != nil {
			return err
		}
		return e.Initialize(ctx, *profile)
	}
	if command == "offline-request" || command == "offline-recover" {
		if !*ack {
			return errors.New("offline recovery requires explicit isolated-target acknowledgement")
		}
		if command == "offline-request" {
			key, err := backup.ReadKey(c.StateKeyFile, true)
			if err != nil {
				return err
			}
			journal, err := backup.OpenJournal(c.StateDir, key)
			if err != nil {
				return err
			}
			defer journal.Close()
			var entry backup.JournalRun
			if err = journal.Read(*runID, &entry); err != nil {
				return err
			}
			b, err := backup.NewOfflineBundle(entry, "offline_"+time.Now().UTC().Format("20060102T150405")+"_"+*target, *copyID, *target, *operation, time.Now())
			if err != nil {
				return err
			}
			return backup.WriteNewProtected(*output, b)
		}
		b, err := backup.ReadOfflineBundle(*bundleFile)
		if err != nil {
			return err
		}
		policyKey, err := backup.ReadKey(c.PolicyPublicKeyFile, false)
		if err != nil {
			return err
		}
		seed, err := backup.ReadKey(c.PrivateKeyFile, true)
		if err != nil {
			return err
		}
		keys := map[string]ed25519.PublicKey{}
		for id, path := range c.RecoveryCustodians {
			raw, err := backup.ReadKey(path, false)
			if err != nil {
				return err
			}
			keys[id] = ed25519.PublicKey(raw)
		}
		engine, err := backup.NewEngine(c)
		if err != nil {
			return err
		}
		target, err := engine.OfflineRecover(ctx, b, ed25519.PublicKey(policyKey), ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey), keys)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"target": target, "state": "materialized", "database_started": false, "financial_execution_allowed": false})
	}
	if command == "status" {
		seed, err := backup.ReadKey(c.PrivateKeyFile, true)
		if err != nil {
			return err
		}
		h, err := backup.CheckHealth(filepath.Join(c.StateDir, "health.json"), ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey), time.Now(), 5*time.Minute, 26*time.Hour)
		message := "healthy"
		if err != nil {
			message = err.Error()
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"health": h, "status": message})
	}
	a, err := backup.OpenAgent(c)
	if err != nil {
		return err
	}
	defer a.Close()
	switch command {
	case "once":
		return a.Tick(ctx)
	case "watch":
		err = a.Watch(ctx, 30*time.Second, func(error) {
			fmt.Fprintln(os.Stderr, "backup_agent_iteration_failed: check policy lease, connectivity and protected journal")
		})
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	default:
		return errors.New("unsupported agent operation")
	}
}
func keygen(dir string) error {
	if err := os.Mkdir(dir, 0700); err != nil {
		return err
	}
	for _, name := range []string{"policy", "agent", "journal"} {
		seed := make([]byte, 32)
		if _, err := rand.Read(seed); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name+".key"), []byte(base64.StdEncoding.EncodeToString(seed)+"\n"), 0600); err != nil {
			return err
		}
		if name != "journal" {
			pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
			if err := os.WriteFile(filepath.Join(dir, name+".pub"), []byte(base64.StdEncoding.EncodeToString(pub)+"\n"), 0644); err != nil {
				return err
			}
		}
	}
	fmt.Println("Created independent keys. Separate policy-signing, agent and journal custody before deployment; nothing was activated.")
	return nil
}
