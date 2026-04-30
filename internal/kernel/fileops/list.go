package fileops

import (
	"os"
	"sort"
	"time"

	serr "github.com/agenthands/helix/internal/errors"
)

// DirEntry represents a single directory entry.
type DirEntry struct {
	Name    string
	IsDir   bool
	Size    int64
	ModTime time.Time
}

// ListDirectory returns directory entries sorted alphabetically with directories first.
func ListDirectory(root, path string) ([]DirEntry, error) {
	absPath, err := ValidatePath(root, path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, serr.New(serr.NotFound, "directory not found").WithDetail(path)
		}
		return nil, serr.Wrap(serr.Internal, "stat directory", err)
	}
	if !info.IsDir() {
		return nil, serr.New(serr.InvalidArgs, "path is not a directory").WithDetail(path)
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, serr.Wrap(serr.Internal, "reading directory", err)
	}

	result := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue // skip entries we can't stat
		}
		result = append(result, DirEntry{
			Name:    e.Name(),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	// Sort: directories first, then alphabetical
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir // dirs first
		}
		return result[i].Name < result[j].Name
	})

	return result, nil
}
