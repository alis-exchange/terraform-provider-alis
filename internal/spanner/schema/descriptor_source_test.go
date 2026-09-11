package schema

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"terraform-provider-alis/internal/utils"
)

type fakeBlobReader map[string][]utils.NamedBlob

func (f fakeBlobReader) ReadAll(_ context.Context, uri string) ([]utils.NamedBlob, error) {
	blobs, ok := f[uri]
	if !ok {
		return nil, errors.New("no objects under " + uri)
	}
	return blobs, nil
}

func blobNames(blobs []utils.NamedBlob) []string {
	names := make([]string, len(blobs))
	for i, b := range blobs {
		names[i] = b.Name
	}
	return names
}

func TestLoadDescriptorSources(t *testing.T) {
	ctx := context.Background()
	fixtures := filepath.Join("testdata", "proto_bundle")
	ownedPath := filepath.Join(fixtures, "owned.fds")

	t.Run("local file", func(t *testing.T) {
		blobs, err := LoadDescriptorSources(ctx, []DescriptorSource{{LocalPath: ownedPath}}, nil)
		if err != nil {
			t.Fatalf("LoadDescriptorSources: %v", err)
		}
		if len(blobs) != 1 || blobs[0].Name != ownedPath || len(blobs[0].Data) == 0 {
			t.Fatalf("blobs = %v, want one blob named %s", blobNames(blobs), ownedPath)
		}
	})

	t.Run("local directory reads every regular file sorted, non-recursive", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{"b.fds", "a.fds"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Mkdir(filepath.Join(dir, "nested"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "nested", "c.fds"), []byte("c"), 0o600); err != nil {
			t.Fatal(err)
		}
		blobs, err := LoadDescriptorSources(ctx, []DescriptorSource{{LocalPath: dir}}, nil)
		if err != nil {
			t.Fatalf("LoadDescriptorSources: %v", err)
		}
		want := []string{filepath.Join(dir, "a.fds"), filepath.Join(dir, "b.fds")}
		if got := blobNames(blobs); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("blobs = %v, want %v", got, want)
		}
	})

	t.Run("directory contents are not filtered by extension", func(t *testing.T) {
		// fds files carry arbitrary names (fds_including_imports), so a stray
		// text file surfaces as a merge error naming it rather than being
		// silently skipped.
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0o600); err != nil {
			t.Fatal(err)
		}
		blobs, err := LoadDescriptorSources(ctx, []DescriptorSource{{LocalPath: dir}}, nil)
		if err != nil {
			t.Fatalf("LoadDescriptorSources: %v", err)
		}
		_, err = MergeDescriptorSets(blobs)
		if err == nil || !strings.Contains(err.Error(), "notes.txt") {
			t.Fatalf("MergeDescriptorSets err = %v, want an error naming notes.txt", err)
		}
	})

	t.Run("gcs object and prefix delegate to the reader", func(t *testing.T) {
		reader := fakeBlobReader{
			"gs://b/one.fds": {{Name: "gs://b/one.fds", Data: []byte("1")}},
			"gs://b/dir/":    {{Name: "gs://b/dir/a", Data: []byte("a")}, {Name: "gs://b/dir/b", Data: []byte("b")}},
		}
		blobs, err := LoadDescriptorSources(ctx, []DescriptorSource{{GcsURI: "gs://b/one.fds"}, {GcsURI: "gs://b/dir/"}}, reader)
		if err != nil {
			t.Fatalf("LoadDescriptorSources: %v", err)
		}
		want := "gs://b/one.fds,gs://b/dir/a,gs://b/dir/b"
		if got := strings.Join(blobNames(blobs), ","); got != want {
			t.Fatalf("blobs = %s, want %s (source order preserved)", got, want)
		}
	})

	t.Run("gcs without a reader is an error", func(t *testing.T) {
		_, err := LoadDescriptorSources(ctx, []DescriptorSource{{GcsURI: "gs://b/one.fds"}}, nil)
		if err == nil || !strings.Contains(err.Error(), "gs://b/one.fds") {
			t.Fatalf("err = %v, want an error naming the URI", err)
		}
	})

	t.Run("exactly one field per source", func(t *testing.T) {
		if _, err := LoadDescriptorSources(ctx, []DescriptorSource{{LocalPath: ownedPath, GcsURI: "gs://b/o"}}, nil); err == nil {
			t.Error("both fields set: expected an error")
		}
		if _, err := LoadDescriptorSources(ctx, []DescriptorSource{{}}, nil); err == nil {
			t.Error("neither field set: expected an error")
		}
	})

	t.Run("missing local path names the path", func(t *testing.T) {
		_, err := LoadDescriptorSources(ctx, []DescriptorSource{{LocalPath: "/nonexistent/x.fds"}}, nil)
		if err == nil || !strings.Contains(err.Error(), "/nonexistent/x.fds") {
			t.Fatalf("err = %v, want an error naming the path", err)
		}
	})
}
