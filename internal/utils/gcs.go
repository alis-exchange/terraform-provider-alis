package utils

import (
	"context"
	"errors"
	"io"
	"net/url"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ReadGcsUri is a utility function to read an object from storage using a URI
// URIs are in the format gs://bucket-name/object-name
func ReadGcsUri(ctx context.Context, uri string) ([]byte, *storage.ObjectAttrs, error) {
	// Check if the URI is valid
	if uri == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "URI is required")
	}

	// Parse the URI
	parsedUri, err := url.Parse(uri)
	if err != nil {
		return nil, nil, status.Error(codes.InvalidArgument, "invalid URI")
	}

	// Check if the URI is a valid GCS URI
	if parsedUri.Scheme != "gs" {
		return nil, nil, status.Error(codes.InvalidArgument, "invalid GCS URI")
	}

	// Get the bucket and object name from the URI
	bucketName := parsedUri.Host
	objectName := strings.TrimLeft(parsedUri.Path, "/")

	// Create new client
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, nil, err
	}

	// Read the object from storage
	bucket := client.Bucket(bucketName)
	o := bucket.Object(objectName)

	attrs, err := o.Attrs(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, nil, status.Error(
				codes.NotFound, objectName)
		} else {
			return nil, nil, err
		}
	}

	rc, err := o.NewReader(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, nil, status.Error(
				codes.NotFound, objectName)
		} else {
			return nil, nil, err
		}
	}
	defer rc.Close()

	// Parse object data
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, nil, err
	}

	return data, attrs, nil
}
