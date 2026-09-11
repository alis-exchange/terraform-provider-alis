package schema

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"terraform-provider-alis/internal/utils"
)

// DescriptorSource names one place FileDescriptorSet bytes come from. Exactly
// one field is set: LocalPath is a file or a directory (direct children only,
// any file name: Define calls its output fds_including_imports), GcsURI is a
// gs://bucket/object or a gs://bucket/prefix/ read through a BlobReader.
type DescriptorSource struct {
	LocalPath string
	GcsURI    string
}

// LoadDescriptorSources reads every source in order and returns the blobs it
// found, each named by the path or URI it came from so later errors can point
// at it. gcs is only needed when a source uses GcsURI; nil is allowed
// otherwise. It does not parse the blobs; see MergeDescriptorSets.
func LoadDescriptorSources(ctx context.Context, sources []DescriptorSource, gcs utils.BlobReader) ([]utils.NamedBlob, error) {
	var blobs []utils.NamedBlob
	for i, src := range sources {
		switch {
		case src.LocalPath != "" && src.GcsURI != "":
			return nil, fmt.Errorf("source %d sets both local_path and gcs_uri; set exactly one", i)
		case src.LocalPath != "":
			local, err := readLocal(src.LocalPath)
			if err != nil {
				return nil, err
			}
			blobs = append(blobs, local...)
		case src.GcsURI != "":
			if gcs == nil {
				return nil, fmt.Errorf("source %d (%s) needs Google Cloud credentials to read from Cloud Storage", i, src.GcsURI)
			}
			remote, err := gcs.ReadAll(ctx, src.GcsURI)
			if err != nil {
				return nil, err
			}
			blobs = append(blobs, remote...)
		default:
			return nil, fmt.Errorf("source %d sets neither local_path nor gcs_uri; set exactly one", i)
		}
	}
	return blobs, nil
}

func readLocal(path string) ([]utils.NamedBlob, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if !info.IsDir() {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		return []utils.NamedBlob{{Name: path, Data: data}}, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var blobs []utils.NamedBlob
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		name := filepath.Join(path, entry.Name())
		data, err := os.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		blobs = append(blobs, utils.NamedBlob{Name: name, Data: data})
	}
	sort.Slice(blobs, func(i, j int) bool { return blobs[i].Name < blobs[j].Name })
	return blobs, nil
}
