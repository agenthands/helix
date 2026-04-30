package upgrade

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	serr "github.com/agenthands/helix/internal/errors"
)

// extractTarGz decompresses archivePath (a .tar.gz file) and writes its
// entries into destDir. Returns a typed error on malformed input.
//
// Zip-slip protection (CRITICAL — see threat model T-52-04-05): every
// entry path is cleaned and the resolved absolute path must be a prefix
// of destDir. An entry whose name escapes destDir via `..` or absolute
// paths is rejected with an error. The check happens BEFORE any file
// I/O so a malicious archive cannot create files outside the stage dir.
//
// Returns a typed Internal error with detail describing the failed entry
// for diagnostics.
func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return serr.Wrap(serr.NotFound, "opening archive", err)
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return serr.Wrap(serr.InvalidArgs, "gzip reader", err)
	}
	defer func() { _ = gz.Close() }()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return serr.Wrap(serr.Internal, "creating destDir", err)
	}
	absDest, err := filepath.Abs(destDir)
	if err != nil {
		return serr.Wrap(serr.Internal, "resolving destDir absolute path", err)
	}

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return serr.Wrap(serr.InvalidArgs, "reading tar header", err)
		}

		// Reject absolute paths outright.
		if filepath.IsAbs(hdr.Name) {
			return serr.New(serr.InvalidArgs, "absolute path in archive entry").
				WithDetail("entry=" + hdr.Name)
		}

		// Resolve and zip-slip-check.
		target := filepath.Join(absDest, hdr.Name)
		clean := filepath.Clean(target)
		if !strings.HasPrefix(clean, absDest+string(os.PathSeparator)) && clean != absDest {
			return serr.New(serr.InvalidArgs, "archive entry escapes destination").
				WithDetail(fmt.Sprintf("entry=%s resolved=%s", hdr.Name, clean))
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(clean, os.FileMode(hdr.Mode)&0o777); err != nil {
				return serr.Wrap(serr.Internal, "mkdir "+clean, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(clean), 0o755); err != nil {
				return serr.Wrap(serr.Internal, "mkdir parent for "+clean, err)
			}
			out, err := os.OpenFile(clean, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return serr.Wrap(serr.Internal, "opening "+clean, err)
			}
			// Limit copy to header-declared size to defend against zip-bombs.
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return serr.Wrap(serr.Internal, "writing "+clean, err)
			}
			if err := out.Close(); err != nil {
				return serr.Wrap(serr.Internal, "closing "+clean, err)
			}
		case tar.TypeSymlink, tar.TypeLink:
			// Symlinks and hard links are explicitly rejected — a malicious
			// archive could symlink "helix" → "/usr/bin/sudo" and trick the
			// later swap step into installing arbitrary content.
			return serr.New(serr.InvalidArgs, "symlinks not allowed in archive").
				WithDetail("entry=" + hdr.Name)
		default:
			// Skip device files, fifos, etc. — Helix archives never legitimately
			// contain them.
			continue
		}
	}
	return nil
}
