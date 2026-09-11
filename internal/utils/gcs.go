package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"cloud.google.com/go/storage"
	googleoauth "golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// NamedBlob is one object's bytes together with the name it was read from,
// so merge conflicts and parse failures can point at a concrete source.
type NamedBlob struct {
	Name string
	Data []byte
}

// BlobReader reads every object a gs:// URI addresses. Behind an interface so
// source-loading logic is testable without a storage client.
type BlobReader interface {
	// ReadAll returns one blob for "gs://bucket/object", or every direct
	// child (no recursion into deeper "folders") for a prefix
	// "gs://bucket/prefix/", sorted by name. A missing object or an empty
	// prefix is an error that names the URI.
	ReadAll(ctx context.Context, uri string) ([]NamedBlob, error)
}

// ParseGcsURI splits gs://bucket/object into its parts. A trailing "/" marks
// a prefix rather than an object.
func ParseGcsURI(uri string) (bucket, object string, isPrefix bool, err error) {
	rest, ok := strings.CutPrefix(uri, "gs://")
	if !ok {
		return "", "", false, fmt.Errorf("gcs uri %q must start with gs://", uri)
	}
	bucket, object, ok = strings.Cut(rest, "/")
	if !ok || bucket == "" || object == "" {
		return "", "", false, fmt.Errorf("gcs uri %q must have the form gs://bucket/object or gs://bucket/prefix/", uri)
	}
	return bucket, object, strings.HasSuffix(object, "/"), nil
}

// GcsReader is the BlobReader backed by Cloud Storage.
type GcsReader struct {
	client *storage.Client
}

var _ BlobReader = (*GcsReader)(nil)

// NewGcsReader builds a reader on the provider's resolved credentials. nil
// credentials fall back to Application Default Credentials; when
// STORAGE_EMULATOR_HOST is set the client ignores credentials entirely, the
// same convention the Spanner adapter follows for its emulator.
func NewGcsReader(ctx context.Context, creds *googleoauth.Credentials) (*GcsReader, error) {
	var opts []option.ClientOption
	if creds != nil {
		opts = append(opts, option.WithCredentials(creds))
	}
	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("storage.NewClient: %w", err)
	}
	return &GcsReader{client: client}, nil
}

// Close releases the underlying client.
func (r *GcsReader) Close() error { return r.client.Close() }

func (r *GcsReader) ReadAll(ctx context.Context, uri string) ([]NamedBlob, error) {
	bucket, object, isPrefix, err := ParseGcsURI(uri)
	if err != nil {
		return nil, err
	}
	bkt := r.client.Bucket(bucket)
	if !isPrefix {
		data, err := r.read(ctx, bkt, object)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", uri, err)
		}
		return []NamedBlob{{Name: uri, Data: data}}, nil
	}

	var blobs []NamedBlob
	it := bkt.Objects(ctx, &storage.Query{Prefix: object, Delimiter: "/"})
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("listing %s: %w", uri, err)
		}
		// Entries for sub-prefixes carry Prefix instead of Name.
		if attrs.Name == "" {
			continue
		}
		data, err := r.read(ctx, bkt, attrs.Name)
		if err != nil {
			return nil, fmt.Errorf("reading gs://%s/%s: %w", bucket, attrs.Name, err)
		}
		blobs = append(blobs, NamedBlob{Name: "gs://" + bucket + "/" + attrs.Name, Data: data})
	}
	if len(blobs) == 0 {
		return nil, fmt.Errorf("no objects under %s", uri)
	}
	sort.Slice(blobs, func(i, j int) bool { return blobs[i].Name < blobs[j].Name })
	return blobs, nil
}

func (r *GcsReader) read(ctx context.Context, bkt *storage.BucketHandle, object string) ([]byte, error) {
	rc, err := bkt.Object(object).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}
