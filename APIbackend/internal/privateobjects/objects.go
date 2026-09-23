// Package privateobjects stores application-encrypted bytes outside the web root.
package privateobjects

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var objectKey = regexp.MustCompile(`^[A-Za-z0-9_-]{10,150}$`)

type Local struct{ Root *os.Root }

func NewLocal(path string) (*Local, error) {
	if e := os.MkdirAll(path, 0700); e != nil {
		return nil, e
	}
	root, e := os.OpenRoot(path)
	return &Local{Root: root}, e
}
func (l *Local) Put(ctx context.Context, key string, data []byte) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if !objectKey.MatchString(key) {
		return errors.New("invalid object key")
	}
	f, e := l.Root.OpenFile(key, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	_, e = f.Write(data)
	if e == nil {
		e = f.Sync()
	}
	return e
}
func (l *Local) Get(ctx context.Context, key string) ([]byte, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if !objectKey.MatchString(key) {
		return nil, errors.New("invalid object key")
	}
	f, e := l.Root.Open(key)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	data, e := io.ReadAll(io.LimitReader(f, (16<<20)+1))
	if len(data) > 16<<20 {
		return nil, errors.New("oversized encrypted object")
	}
	return data, e
}
func (l *Local) Delete(ctx context.Context, key string) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if !objectKey.MatchString(key) {
		return errors.New("invalid object key")
	}
	e := l.Root.Remove(key)
	if os.IsNotExist(e) {
		return nil
	}
	return e
}

// LocalScanner deliberately does not claim antivirus acceptance.
type LocalScanner struct{}

func (LocalScanner) Scan(context.Context, []byte) (string, error) { return "local_unscanned", nil }

type S3 struct {
	Client *minio.Client
	Bucket string
}

func NewS3(endpoint, bucket, region, key, secret string) (*S3, error) {
	if endpoint == "" || bucket == "" || region == "" || key == "" || secret == "" || strings.ContainsAny(endpoint, "/?#@") || !regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`).MatchString(bucket) {
		return nil, errors.New("explicit private S3 endpoint, bucket, region and credentials required")
	}
	c, e := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(key, secret, ""), Secure: true, Region: region, BucketLookup: minio.BucketLookupPath})
	if e != nil {
		return nil, e
	}
	return &S3{c, bucket}, nil
}
func (s *S3) VerifyPrivate(ctx context.Context) error {
	policy, e := s.Client.GetBucketPolicy(ctx, s.Bucket)
	if e != nil {
		code := minio.ToErrorResponse(e).Code
		if code != "NoSuchBucketPolicy" {
			return errors.New("cannot verify private bucket policy")
		}
	}
	if strings.TrimSpace(policy) != "" && strings.TrimSpace(policy) != "{}" {
		return errors.New("bucket policies require separate review; use a dedicated bucket without a public policy")
	}
	return nil
}
func (s *S3) Put(ctx context.Context, key string, data []byte) error {
	if !objectKey.MatchString(key) {
		return errors.New("invalid object key")
	}
	_, e := s.Client.PutObject(ctx, s.Bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: "application/octet-stream", DisableMultipart: true})
	return e
}
func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	if !objectKey.MatchString(key) {
		return nil, errors.New("invalid object key")
	}
	obj, e := s.Client.GetObject(ctx, s.Bucket, key, minio.GetObjectOptions{})
	if e != nil {
		return nil, e
	}
	defer obj.Close()
	data, e := io.ReadAll(io.LimitReader(obj, (16<<20)+1))
	if len(data) > 16<<20 {
		return nil, errors.New("oversized encrypted object")
	}
	return data, e
}
func (s *S3) Delete(ctx context.Context, key string) error {
	if !objectKey.MatchString(key) {
		return errors.New("invalid object key")
	}
	return s.Client.RemoveObject(ctx, s.Bucket, key, minio.RemoveObjectOptions{})
}

// Clamd uses a local Unix socket: no unauthenticated antivirus TCP port is exposed.
type Clamd struct{ Socket string }

func (c Clamd) Scan(ctx context.Context, data []byte) (string, error) {
	if !strings.HasPrefix(c.Socket, "/") {
		return "", errors.New("absolute ClamAV Unix socket required")
	}
	dial := net.Dialer{Timeout: 5 * time.Second}
	conn, e := dial.DialContext(ctx, "unix", c.Socket)
	if e != nil {
		return "", e
	}
	defer conn.Close()
	deadline := time.Now().Add(30 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	if _, e = io.Copy(conn, bytes.NewReader([]byte("zINSTREAM\x00"))); e != nil {
		return "", e
	}
	for offset := 0; offset < len(data); offset += 32768 {
		end := offset + 32768
		if end > len(data) {
			end = len(data)
		}
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(end-offset))
		if _, e = io.Copy(conn, bytes.NewReader(size[:])); e != nil {
			return "", e
		}
		if _, e = io.Copy(conn, bytes.NewReader(data[offset:end])); e != nil {
			return "", e
		}
	}
	if _, e = io.Copy(conn, bytes.NewReader([]byte{0, 0, 0, 0})); e != nil {
		return "", e
	}
	answer, e := bufio.NewReader(io.LimitReader(conn, 4096)).ReadString('\x00')
	if e != nil {
		return "", errors.New("incomplete antivirus response")
	}
	if answer != "stream: OK\x00" {
		return "rejected", nil
	}
	return "clean", nil
}
