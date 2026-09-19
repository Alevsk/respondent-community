package declarative

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/Alevsk/respondent/internal/logging"
)

// defaultS3Region is used when no region is specified in the S3PollSpec.
const defaultS3Region = "us-east-1"

// Compile-time assertion that S3PollTransport implements Transport (pull-based only).
var _ Transport = (*S3PollTransport)(nil)

func init() {
	RegisterTransport("s3_poll", newS3PollTransport)
}

// s3Client abstracts the MinIO client operations used by S3PollTransport.
// This enables testing without connecting to a real S3/MinIO endpoint.
type s3Client interface {
	ListObjects(ctx context.Context, bucket string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	GetObject(ctx context.Context, bucket, object string, opts minio.GetObjectOptions) (*minio.Object, error)
	RemoveObject(ctx context.Context, bucket, object string, opts minio.RemoveObjectOptions) error
}

// S3PollTransport implements Transport for S3-compatible object storage polling.
// Each call to Fetch lists objects matching the configured prefix, file pattern,
// and time window, downloads their contents, and returns them as a concatenated
// JSON array.
type S3PollTransport struct {
	spec       *S3PollSpec
	sourceName string
	region     string
	accessKey  string
	secretKey  string
	logger     *logging.Logger
	client     s3Client
}

// newS3PollTransport creates an S3PollTransport from a source definition.
func newS3PollTransport(def *SourceDefinition, _ map[string]string, _ TokenProvider, envResolve EnvResolver, logger *logging.Logger) (Transport, error) {
	if def.Transport.S3Poll == nil {
		return nil, fmt.Errorf("s3_poll transport requires transport.s3_poll configuration")
	}

	spec := def.Transport.S3Poll

	if spec.Endpoint == "" {
		return nil, fmt.Errorf("s3_poll transport requires an endpoint")
	}
	if spec.Bucket == "" {
		return nil, fmt.Errorf("s3_poll transport requires a bucket")
	}

	region := spec.Region
	if region == "" {
		region = defaultS3Region
	}

	accessKey := resolveCredential(spec.AccessKey, envResolve)
	secretKey := resolveCredential(spec.SecretKey, envResolve)

	useSSL := !spec.PathStyle // default: virtual-hosted style implies HTTPS

	client, err := minio.New(spec.Endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure:       useSSL,
		Region:       region,
		BucketLookup: bucketLookup(spec.PathStyle),
	})
	if err != nil {
		return nil, fmt.Errorf("create S3 client: %w", err)
	}

	return &S3PollTransport{
		spec:       spec,
		sourceName: def.Name,
		region:     region,
		accessKey:  accessKey,
		secretKey:  secretKey,
		logger:     logger,
		client:     client,
	}, nil
}

// Fetch lists objects in the configured bucket matching prefix, pattern, and
// time filters, downloads each, and returns the concatenated contents as a
// JSON array: [obj1, obj2, ...]. The method and url parameters are ignored
// because configuration comes from the S3PollSpec.
func (t *S3PollTransport) Fetch(ctx context.Context, _, _ string) ([]byte, int, error) {
	opts := minio.ListObjectsOptions{
		Prefix:    t.spec.Prefix,
		Recursive: true,
	}

	var cutoff time.Time
	if t.spec.SinceLastModified.Duration > 0 {
		cutoff = time.Now().Add(-t.spec.SinceLastModified.Duration)
	}

	var objects []minio.ObjectInfo
	for obj := range t.client.ListObjects(ctx, t.spec.Bucket, opts) {
		if obj.Err != nil {
			return nil, 0, fmt.Errorf("list objects in bucket %q: %w", t.spec.Bucket, obj.Err)
		}

		// Time filter: skip objects older than the window.
		if !cutoff.IsZero() && obj.LastModified.Before(cutoff) {
			continue
		}

		// Pattern filter: match object key against glob.
		if t.spec.FilePattern != "" {
			base := filepath.Base(obj.Key)
			matched, err := filepath.Match(t.spec.FilePattern, base)
			if err != nil {
				return nil, 0, fmt.Errorf("invalid file_pattern %q: %w", t.spec.FilePattern, err)
			}
			if !matched {
				continue
			}
		}

		objects = append(objects, obj)
	}

	if len(objects) == 0 {
		// Return empty JSON array when no objects match.
		return []byte("[]"), 200, nil
	}

	// Download and concatenate all matching objects into a JSON array.
	var buf bytes.Buffer
	buf.WriteByte('[')

	for i, obj := range objects {
		reader, err := t.client.GetObject(ctx, t.spec.Bucket, obj.Key, minio.GetObjectOptions{})
		if err != nil {
			return nil, 0, fmt.Errorf("get object %q: %w", obj.Key, err)
		}

		data, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			return nil, 0, fmt.Errorf("read object %q: %w", obj.Key, err)
		}

		if i > 0 {
			buf.WriteByte(',')
		}
		buf.Write(data)

		t.logger.Debug("fetched S3 object",
			logging.String("source_name", t.sourceName),
			logging.String("key", obj.Key),
			logging.Int("size", len(data)),
		)

		// Delete after fetch if configured.
		if t.spec.DeleteAfterFetch {
			if err := t.client.RemoveObject(ctx, t.spec.Bucket, obj.Key, minio.RemoveObjectOptions{}); err != nil {
				t.logger.Warn("failed to delete S3 object after fetch",
					logging.String("source_name", t.sourceName),
					logging.String("key", obj.Key),
					logging.Err("error", err),
				)
			}
		}
	}

	buf.WriteByte(']')

	t.logger.Info("s3_poll fetch complete",
		logging.String("source_name", t.sourceName),
		logging.String("bucket", t.spec.Bucket),
		logging.Int("objects_fetched", len(objects)),
	)

	return buf.Bytes(), 200, nil
}

// resolveCredential resolves a credential value through the EnvResolver.
// If the value is empty, returns empty string (anonymous access).
func resolveCredential(value string, envResolve EnvResolver) string {
	if value == "" {
		return ""
	}
	return envResolve(value)
}

// bucketLookup returns the appropriate BucketLookupType based on path style config.
func bucketLookup(pathStyle bool) minio.BucketLookupType {
	if pathStyle {
		return minio.BucketLookupPath
	}
	return minio.BucketLookupAuto
}
