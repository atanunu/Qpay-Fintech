package backup

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func (e *Engine) objectClient(profile string) (*minio.Client, ObjectSourceProfile, error) {
	p, ok := e.Config.ObjectSources[profile]
	if !ok {
		return nil, p, errors.New("private object source is not provisioned")
	}
	u, err := url.Parse(p.Endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || u.Scheme != "https" && (e.Config.Environment != "local" || u.Scheme != "http" || u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1") {
		return nil, p, errors.New("object source requires an exact HTTPS endpoint")
	}
	if p.Bucket == "" || strings.Contains(p.Prefix, "..") || strings.HasPrefix(p.Prefix, "/") || e.Config.Environment != "local" && !p.RequireVersioning {
		return nil, p, errors.New("approved bucket/prefix and non-local versioning are required")
	}
	env, err := ReadEnvironment(p.EnvironmentFile)
	if err != nil {
		return nil, p, err
	}
	if env["AWS_ACCESS_KEY_ID"] == "" || env["AWS_SECRET_ACCESS_KEY"] == "" {
		return nil, p, errors.New("object source credentials missing")
	}
	c, err := minio.New(u.Host, &minio.Options{Creds: credentials.NewStaticV4(env["AWS_ACCESS_KEY_ID"], env["AWS_SECRET_ACCESS_KEY"], env["AWS_SESSION_TOKEN"]), Secure: u.Scheme == "https", Region: p.Region})
	return c, p, err
}
func (e *Engine) captureObjectVersions(ctx context.Context, profile string, keys []string, dir string, budget *int64) (map[string]string, error) {
	versions := map[string]string{}
	client, p, err := e.objectClient(profile)
	if err != nil {
		return nil, err
	}
	if err = os.Mkdir(filepath.Join(dir, "private-objects"), 0700); err != nil {
		return nil, err
	}
	for _, key := range keys {
		if !ID(key) {
			return nil, errors.New("invalid private object key")
		}
		remote := path.Join(p.Prefix, key)
		info, err := client.StatObject(ctx, p.Bucket, remote, minio.StatObjectOptions{})
		if err != nil {
			return nil, errors.New("required private object is missing")
		}
		if info.Size < 0 || info.Size > *budget || p.RequireVersioning && (info.VersionID == "" || info.VersionID == "null") {
			return nil, errors.New("required private object is unversioned or exceeds staging capacity")
		}
		object, err := client.GetObject(ctx, p.Bucket, remote, minio.GetObjectOptions{VersionID: info.VersionID})
		if err != nil {
			return nil, errors.New("private object version cannot be read")
		}
		size, _, copyErr := saveStream(filepath.Join(dir, "private-objects", key), object, budget)
		object.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if size != info.Size {
			return nil, errors.New("private object length changed")
		}
		versions["private-objects/"+key] = info.VersionID
	}
	return versions, nil
}
