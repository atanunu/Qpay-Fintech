package backup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionedPrivateObjectCapturePinsTheActualS3Version(t *testing.T) {
	var seenVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/evidence/objects/obj_test" {
			t.Error(r.URL.Path)
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Length", "8")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Last-Modified", "Thu, 24 Sep 2026 00:00:00 GMT")
		w.Header().Set("ETag", `"deadbeef"`)
		w.Header().Set("X-Amz-Version-Id", "exact-version-one")
		if r.Method == "HEAD" {
			return
		}
		seenVersion = r.URL.Query().Get("versionId")
		w.Write([]byte("ciphered"))
	}))
	defer server.Close()
	dir := t.TempDir()
	env := filepath.Join(dir, "environment.json")
	os.WriteFile(env, Canonical(map[string]string{"AWS_ACCESS_KEY_ID": "synthetic", "AWS_SECRET_ACCESS_KEY": "synthetic-key-only"}), 0600)
	e := Engine{Config: AgentConfig{Environment: "local", ObjectSources: map[string]ObjectSourceProfile{"evidence": {Endpoint: server.URL, Region: "us-east-1", Bucket: "evidence", Prefix: "objects", EnvironmentFile: env, RequireVersioning: true}}}}
	capture := filepath.Join(dir, "capture")
	os.Mkdir(capture, 0700)
	budget := int64(16)
	versions, err := e.captureObjectVersions(context.Background(), "evidence", []string{"obj_test"}, capture, &budget)
	if err != nil {
		t.Fatal(err)
	}
	if seenVersion != "exact-version-one" || versions["private-objects/obj_test"] != seenVersion {
		t.Fatal("unpinned version", versions, seenVersion)
	}
	b, _ := os.ReadFile(filepath.Join(capture, "private-objects/obj_test"))
	if string(b) != "ciphered" || budget != 8 {
		t.Fatal(string(b), budget)
	}
	p := e.Config.ObjectSources["evidence"]
	p.Endpoint = "http://metadata.example.invalid"
	e.Config.ObjectSources["evidence"] = p
	if _, _, err = e.objectClient("evidence"); err == nil {
		t.Fatal("unsafe object endpoint")
	}
}
func TestPrivateObjectEndpointRejectsCredentialURLsAndNonLocalUnversionedSource(t *testing.T) {
	for _, endpoint := range []string{"https://user:secret@storage.example.invalid", "https://storage.example.invalid/path", "https://storage.example.invalid?secret=value", "http://169.254.169.254"} {
		e := Engine{Config: AgentConfig{Environment: "production", ObjectSources: map[string]ObjectSourceProfile{"objects": {Endpoint: endpoint}}}}
		if _, _, err := e.objectClient("objects"); err == nil {
			t.Fatal("accepted", strings.Split(endpoint, "@")[0])
		}
	}
}
