package fileops

import (
	"bufio"
	"fmt"
	"os"

	serr "github.com/agenthands/helix/internal/errors"
)

// maxFileSize is the maximum file size for ReadFile (10 MB).
const maxFileSize = 10 * 1024 * 1024

// ReadFile reads the entire file at the given path and returns its content as a string.
// Returns an error if the file does not exist or exceeds 10 MB.
func ReadFile(root, path string) (string, error) {
	absPath, err := ValidatePath(root, path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", serr.New(serr.NotFound, "file not found").WithDetail(path)
		}
		return "", serr.Wrap(serr.Internal, "stat file", err)
	}
	if info.IsDir() {
		return "", serr.New(serr.InvalidArgs, "path is a directory").WithDetail(path)
	}
	if info.Size() > maxFileSize {
		return "", serr.New(serr.InvalidArgs, "file exceeds size limit").WithDetail(fmt.Sprintf("%d bytes: %s", info.Size(), path))
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", serr.Wrap(serr.Internal, "reading file", err)
	}
	return string(data), nil
}

// ReadFileRange reads lines from startLine to endLine (1-indexed, inclusive).
// If endLine is 0 or beyond EOF, reads to end of file.
// Returns lines with line numbers prefixed (e.g., "  1: content").
func ReadFileRange(root, path string, startLine, endLine int) (string, error) {
	absPath, err := ValidatePath(root, path)
	if err != nil {
		return "", err
	}

	if startLine < 1 {
		startLine = 1
	}

	f, err := os.Open(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", serr.New(serr.NotFound, "file not found").WithDetail(path)
		}
		return "", serr.Wrap(serr.Internal, "opening file", err)
	}
	defer f.Close()

	var result string
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum < startLine {
			continue
		}
		if endLine > 0 && lineNum > endLine {
			break
		}
		result += fmt.Sprintf("%4d: %s\n", lineNum, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", serr.Wrap(serr.Internal, "scanning file", err)
	}

	if result == "" && startLine > lineNum {
		return "", serr.New(serr.InvalidArgs, "start line beyond end of file").WithDetail(fmt.Sprintf("line %d, file has %d lines", startLine, lineNum))
	}

	return result, nil
}
