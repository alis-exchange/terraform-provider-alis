package spanner

import (
	"strings"
	"testing"
)

func TestProtoTimestampDdl(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		want      string
		wantErr   bool
	}{
		{
			name:      "proto field path",
			fieldPath: "Book.create_time",
			want:      "TIMESTAMP_ADD(TIMESTAMP_SECONDS(Book.create_time.seconds),INTERVAL CAST(FLOOR(Book.create_time.nanos / 1000) AS INT64) MICROSECOND)",
		},
		{
			name:      "deeply nested field",
			fieldPath: "Book.metadata.update_time",
			want:      "TIMESTAMP_ADD(TIMESTAMP_SECONDS(Book.metadata.update_time.seconds),INTERVAL CAST(FLOOR(Book.metadata.update_time.nanos / 1000) AS INT64) MICROSECOND)",
		},
		{name: "empty", fieldPath: "", wantErr: true},
		{name: "sql injection", fieldPath: "Book.name)) STORED; DROP TABLE x --", wantErr: true},
		{name: "leading digit", fieldPath: "1Book.name", wantErr: true},
		{name: "trailing dot", fieldPath: "Book.", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := protoTimestampDdl(tc.fieldPath)
			if (err != nil) != tc.wantErr {
				t.Fatalf("protoTimestampDdl(%q) error = %v, wantErr %v", tc.fieldPath, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("protoTimestampDdl(%q) = %q, want %q", tc.fieldPath, got, tc.want)
			}
		})
	}
}

func TestResourceNameAncestorDdl(t *testing.T) {
	tests := []struct {
		name        string
		fieldPath   string
		collections []string
		want        string
		wantErr     bool
	}{
		{
			name:        "single collection",
			fieldPath:   "Book.name",
			collections: []string{"shelves"},
			want:        "REGEXP_EXTRACT(Book.name, r'^(shelves/[^/]+)')",
		},
		{
			name:        "two collections",
			fieldPath:   "Book.name",
			collections: []string{"shelves", "books"},
			want:        "REGEXP_EXTRACT(Book.name, r'^(shelves/[^/]+/books/[^/]+)')",
		},
		{
			name:        "plain string column",
			fieldPath:   "name",
			collections: []string{"projects"},
			want:        "REGEXP_EXTRACT(name, r'^(projects/[^/]+)')",
		},
		{name: "no collections", fieldPath: "Book.name", collections: nil, wantErr: true},
		{name: "bad field path", fieldPath: "Book .name", collections: []string{"shelves"}, wantErr: true},
		{name: "bad collection", fieldPath: "Book.name", collections: []string{"she/lves"}, wantErr: true},
		{name: "regex metacharacters", fieldPath: "Book.name", collections: []string{"she.*"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resourceNameAncestorDdl(tc.fieldPath, tc.collections)
			if (err != nil) != tc.wantErr {
				t.Fatalf("resourceNameAncestorDdl(%q, %v) error = %v, wantErr %v", tc.fieldPath, tc.collections, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("resourceNameAncestorDdl(%q, %v) = %q, want %q", tc.fieldPath, tc.collections, got, tc.want)
			}
			if err == nil && strings.Count(got, "(") != strings.Count(got, ")") {
				t.Errorf("unbalanced parentheses in %q", got)
			}
		})
	}
}

func TestResourceNameIdDdl(t *testing.T) {
	tests := []struct {
		name       string
		fieldPath  string
		collection string
		want       string
		wantErr    bool
	}{
		{
			name:       "book id",
			fieldPath:  "Book.name",
			collection: "books",
			want:       "REGEXP_EXTRACT(Book.name, r'books/([^/]+)')",
		},
		{
			name:       "root collection id",
			fieldPath:  "name",
			collection: "shelves",
			want:       "REGEXP_EXTRACT(name, r'shelves/([^/]+)')",
		},
		{name: "bad collection", fieldPath: "Book.name", collection: "she lves", wantErr: true},
		{name: "empty collection", fieldPath: "Book.name", collection: "", wantErr: true},
		{name: "bad field path", fieldPath: "Book.name'", collection: "shelves", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resourceNameIDDdl(tc.fieldPath, tc.collection)
			if (err != nil) != tc.wantErr {
				t.Fatalf("resourceNameIDDdl(%q, %q) error = %v, wantErr %v", tc.fieldPath, tc.collection, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("resourceNameIDDdl(%q, %q) = %q, want %q", tc.fieldPath, tc.collection, got, tc.want)
			}
		})
	}
}
