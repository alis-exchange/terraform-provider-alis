package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseGcsURI(t *testing.T) {
	cases := []struct {
		uri                string
		wantBucket, wantOb string
		wantPrefix         bool
		wantErr            bool
	}{
		{uri: "gs://b/pkg/fds_including_imports", wantBucket: "b", wantOb: "pkg/fds_including_imports"},
		{uri: "gs://b/pkg/", wantBucket: "b", wantOb: "pkg/", wantPrefix: true},
		{uri: "b/o", wantErr: true},
		{uri: "gs://", wantErr: true},
		{uri: "gs://b", wantErr: true},
		{uri: "gs://b/", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.uri, func(t *testing.T) {
			bucket, object, isPrefix, err := ParseGcsURI(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseGcsURI(%q) = %q, %q, %v; want error", tc.uri, bucket, object, isPrefix)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseGcsURI(%q): %v", tc.uri, err)
			}
			if bucket != tc.wantBucket || object != tc.wantOb || isPrefix != tc.wantPrefix {
				t.Fatalf("ParseGcsURI(%q) = %q, %q, %v; want %q, %q, %v",
					tc.uri, bucket, object, isPrefix, tc.wantBucket, tc.wantOb, tc.wantPrefix)
			}
		})
	}
}

// fakeGcs serves the two JSON-API shapes the reader uses: an object listing
// under a prefix, and object media downloads. Media is served on both the
// XML path (/{bucket}/{object}) and the JSON path
// (/download/storage/v1/b/{bucket}/o/{object}) so the test does not depend
// on which one the client picks for the emulator host.
func fakeGcs(t *testing.T, objects map[string][]byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/storage/v1/b/b/o", func(w http.ResponseWriter, r *http.Request) {
		prefix := r.URL.Query().Get("prefix")
		delimiter := r.URL.Query().Get("delimiter")
		type item struct {
			Name   string `json:"name"`
			Bucket string `json:"bucket"`
		}
		var items []item
		var prefixes []string
		for name := range objects {
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			rest := strings.TrimPrefix(name, prefix)
			if delimiter != "" && strings.Contains(rest, delimiter) {
				prefixes = append(prefixes, prefix+strings.SplitN(rest, delimiter, 2)[0]+delimiter)
				continue
			}
			items = append(items, item{Name: name, Bucket: "b"})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"kind": "storage#objects", "items": items, "prefixes": prefixes})
	})
	serve := func(w http.ResponseWriter, name string) {
		data, ok := objects[name]
		if !ok {
			http.NotFound(w, nil)
			return
		}
		_, _ = w.Write(data)
	}
	mux.HandleFunc("/download/storage/v1/b/b/o/", func(w http.ResponseWriter, r *http.Request) {
		serve(w, strings.TrimPrefix(r.URL.Path, "/download/storage/v1/b/b/o/"))
	})
	mux.HandleFunc("/b/", func(w http.ResponseWriter, r *http.Request) {
		serve(w, strings.TrimPrefix(r.URL.Path, "/b/"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestGcsReader_ReadAll(t *testing.T) {
	srv := fakeGcs(t, map[string][]byte{
		"pkg/a.fds":        []byte("aaa"),
		"pkg/b.fds":        []byte("bbb"),
		"pkg/nested/c.fds": []byte("ccc"),
		"other/d.fds":      []byte("ddd"),
	})
	t.Setenv("STORAGE_EMULATOR_HOST", strings.TrimPrefix(srv.URL, "http://"))

	reader, err := NewGcsReader(t.Context(), nil)
	if err != nil {
		t.Fatalf("NewGcsReader: %v", err)
	}
	t.Cleanup(func() { _ = reader.Close() })

	t.Run("single object", func(t *testing.T) {
		blobs, err := reader.ReadAll(t.Context(), "gs://b/pkg/a.fds")
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if len(blobs) != 1 || blobs[0].Name != "gs://b/pkg/a.fds" || string(blobs[0].Data) != "aaa" {
			t.Fatalf("blobs = %+v, want one blob named by its URI with body aaa", blobs)
		}
	})

	t.Run("prefix lists direct children sorted", func(t *testing.T) {
		blobs, err := reader.ReadAll(t.Context(), "gs://b/pkg/")
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if len(blobs) != 2 {
			t.Fatalf("got %d blobs, want 2 direct children (nested/ excluded): %+v", len(blobs), blobs)
		}
		if blobs[0].Name != "gs://b/pkg/a.fds" || blobs[1].Name != "gs://b/pkg/b.fds" {
			t.Fatalf("names = %q, %q; want sorted a then b", blobs[0].Name, blobs[1].Name)
		}
		if string(blobs[0].Data) != "aaa" || string(blobs[1].Data) != "bbb" {
			t.Fatalf("bodies = %q, %q; want aaa, bbb", blobs[0].Data, blobs[1].Data)
		}
	})

	t.Run("missing object names the URI", func(t *testing.T) {
		_, err := reader.ReadAll(t.Context(), "gs://b/pkg/missing.fds")
		if err == nil || !strings.Contains(err.Error(), "gs://b/pkg/missing.fds") {
			t.Fatalf("err = %v, want an error naming the URI", err)
		}
	})

	t.Run("empty prefix is an error", func(t *testing.T) {
		if _, err := reader.ReadAll(t.Context(), "gs://b/none/"); err == nil {
			t.Fatal("expected an error for a prefix with no objects")
		}
	})
}
