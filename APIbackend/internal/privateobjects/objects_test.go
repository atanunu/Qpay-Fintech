package privateobjects

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalStoreOwnerOnlyImmutableAndConfined(t *testing.T) {
	root := t.TempDir()
	store, e := NewLocal(root)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Root.Close()
	key := "synthetic_object_001"
	data := []byte("application-encrypted-bytes")
	if e = store.Put(context.Background(), key, data); e != nil {
		t.Fatal(e)
	}
	if e = store.Put(context.Background(), key, []byte("replace")); e == nil {
		t.Fatal("overwritten immutable object")
	}
	got, e := store.Get(context.Background(), key)
	if e != nil || !bytes.Equal(got, data) {
		t.Fatal(got, e)
	}
	info, e := os.Stat(filepath.Join(root, key))
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("unsafe object permissions", e)
	}
	for _, bad := range []string{"../outside", "/etc/passwd", "a/b", "short"} {
		if e = store.Put(context.Background(), bad, data); e == nil {
			t.Fatal("unsafe key", bad)
		}
		if _, e = store.Get(context.Background(), bad); e == nil {
			t.Fatal("unsafe read", bad)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = store.Get(ctx, key); e == nil {
		t.Fatal("cancelled read accepted")
	}
	if e = store.Delete(context.Background(), key); e != nil {
		t.Fatal(e)
	}
	if e = store.Delete(context.Background(), key); e != nil {
		t.Fatal("delete not idempotent", e)
	}
}
func TestClamdFramingAndFragmentedResponse(t *testing.T) {
	for _, tc := range []struct {
		name, response, want string
		fail                 bool
	}{{"clean", "stream: OK\x00", "clean", false}, {"rejected", "stream: Test FOUND\x00", "rejected", false}, {"incomplete", "stream: OK", "", true}} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "clam.sock")
			listener, e := net.Listen("unix", path)
			if e != nil {
				t.Fatal(e)
			}
			defer listener.Close()
			data := bytes.Repeat([]byte("synthetic"), 9000)
			done := make(chan error, 1)
			go func() {
				conn, e := listener.Accept()
				if e != nil {
					done <- e
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				cmd := make([]byte, 10)
				if _, e = io.ReadFull(conn, cmd); e != nil {
					done <- e
					return
				}
				if string(cmd) != "zINSTREAM\x00" {
					done <- io.ErrUnexpectedEOF
					return
				}
				var received []byte
				for {
					var b [4]byte
					if _, e = io.ReadFull(conn, b[:]); e != nil {
						done <- e
						return
					}
					n := binary.BigEndian.Uint32(b[:])
					if n == 0 {
						break
					}
					if n > 32768 {
						done <- io.ErrShortBuffer
						return
					}
					chunk := make([]byte, n)
					if _, e = io.ReadFull(conn, chunk); e != nil {
						done <- e
						return
					}
					received = append(received, chunk...)
				}
				if !bytes.Equal(data, received) {
					done <- io.ErrUnexpectedEOF
					return
				}
				for i := range len(tc.response) {
					if _, e = conn.Write([]byte{tc.response[i]}); e != nil {
						done <- e
						return
					}
				}
				done <- nil
			}()
			got, e := (Clamd{Socket: path}).Scan(context.Background(), data)
			if (e != nil) != tc.fail || got != tc.want {
				t.Fatalf("got %q %v", got, e)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestUploadIntegrationsFailClosed(t *testing.T) {
	if _, e := (Clamd{Socket: "relative"}).Scan(context.Background(), nil); e == nil {
		t.Fatal("relative scanner socket accepted")
	}
	if _, e := (Clamd{Socket: filepath.Join(t.TempDir(), "missing")}).Scan(context.Background(), nil); e == nil {
		t.Fatal("missing scanner accepted")
	}
	if _, e := NewS3("https://public.invalid/path", "bucket", "region", "key", "secret"); e == nil {
		t.Fatal("unsafe S3 endpoint accepted")
	}
	state, e := (LocalScanner{}).Scan(context.Background(), nil)
	if e != nil || state != "local_unscanned" {
		t.Fatal("local scanner claimed malware qualification")
	}
}
