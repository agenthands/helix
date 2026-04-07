package fileops

import (
	"bufio"
	"fmt"
	"os"
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
			return "", fmt.Errorf("file not found: %s", path)
		}
		return "", fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory: %s", path)
	}
	if info.Size() > maxFileSize {
		return "", fmt.Errorf("file exceeds 10MB size limit (%d bytes): %s", info.Size(), path)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
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
			return "", fmt.Errorf("file not found: %s", path)
		}
		return "", fmt.Errorf("opening file: %w", err)
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
		return "", fmt.Errorf("scanning file: %w", err)
	}

	if result == "" && startLine > lineNum {
		return "", fmt.Errorf("start line %d is beyond end of file (%d lines)", startLine, lineNum)
	}

	return result, nil
}
