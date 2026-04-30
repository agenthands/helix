package langregistry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// InstallerConfig controls the three-tier LS resolution behavior.
type InstallerConfig struct {
	// AutoInstall enables managed download when a binary is not found in PATH.
	// Default: true.
	AutoInstall bool
	// BinDir is the directory for managed installs (default ~/.helix/bin/).
	BinDir string
}

// Installer resolves language server binaries using a three-tier strategy:
// 1. PATH lookup
// 2. Managed download (if AutoInstall and InstallInfo present)
// 3. Helpful error message
type Installer struct {
	config InstallerConfig
	logger *slog.Logger
}

// NewInstaller creates an Installer with the given configuration.
func NewInstaller(cfg InstallerConfig, logger *slog.Logger) *Installer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Installer{config: cfg, logger: logger}
}

// Resolve finds or installs the language server for the given entry.
// It returns the resolved command path and arguments.
func (i *Installer) Resolve(ctx context.Context, entry LSEntry) (command string, args []string, err error) {
	// Tier 1: Check PATH.
	if path, lookErr := exec.LookPath(entry.Command); lookErr == nil {
		i.logger.Debug("language server found in PATH", "language", entry.Language, "path", path)
		return path, entry.Args, nil
	}

	// Tier 2: Managed download if enabled and install info is available.
	if i.config.AutoInstall && entry.Install != nil {
		i.logger.Info("attempting managed install", "language", entry.Language, "type", entry.Install.Type)
		installed, installErr := i.download(ctx, entry)
		if installErr == nil {
			return installed, entry.Args, nil
		}
		i.logger.Warn("managed install failed", "language", entry.Language, "error", installErr)
		// Fall through to Tier 3.
	}

	// Tier 3: Helpful error.
	return "", nil, fmt.Errorf("language server %q not found for %q; install with: %s",
		entry.Command, entry.Language, entry.InstallHint())
}

// PlatformKey returns the current platform key for binary downloads (e.g. "darwin-arm64").
func PlatformKey() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

// download attempts to install a language server using its InstallInfo.
func (i *Installer) download(ctx context.Context, entry LSEntry) (string, error) {
	if entry.Install == nil {
		return "", fmt.Errorf("no install info for %q", entry.Language)
	}

	switch entry.Install.Type {
	case "npm":
		return i.installNPM(ctx, entry)
	case "pip":
		return i.installPip(ctx, entry)
	case "cargo":
		return i.installCargo(ctx, entry)
	case "gem":
		return i.installGem(ctx, entry)
	case "dotnet":
		return i.installDotnet(ctx, entry)
	case "binary":
		return i.installBinary(ctx, entry)
	default:
		return "", fmt.Errorf("unsupported install type %q for %q", entry.Install.Type, entry.Language)
	}
}

func (i *Installer) installNPM(ctx context.Context, entry LSEntry) (string, error) {
	pkg := entry.Install.Package
	if entry.Install.Version != "" {
		pkg += "@" + entry.Install.Version
	}
	cmd := exec.CommandContext(ctx, "npm", "install", "-g", pkg)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("npm install failed for %s: %w", pkg, err)
	}
	return exec.LookPath(entry.Command)
}

func (i *Installer) installPip(ctx context.Context, entry LSEntry) (string, error) {
	pkg := entry.Install.Package
	if entry.Install.Version != "" {
		pkg += "==" + entry.Install.Version
	}
	// Try pipx first (recommended), fall back to pip.
	pipx, pipxErr := exec.LookPath("pipx")
	if pipxErr == nil {
		cmd := exec.CommandContext(ctx, pipx, "install", pkg)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err == nil {
			if path, lookErr := exec.LookPath(entry.Command); lookErr == nil {
				return path, nil
			}
		}
	}
	// Fall back to pip.
	cmd := exec.CommandContext(ctx, "pip", "install", pkg)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pip install failed for %s: %w", pkg, err)
	}
	return exec.LookPath(entry.Command)
}

func (i *Installer) installCargo(ctx context.Context, entry LSEntry) (string, error) {
	args := []string{"install", entry.Install.Package}
	if entry.Install.Version != "" {
		args = append(args, "--version", entry.Install.Version)
	}
	cmd := exec.CommandContext(ctx, "cargo", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("cargo install failed for %s: %w", entry.Install.Package, err)
	}
	return exec.LookPath(entry.Command)
}

func (i *Installer) installGem(ctx context.Context, entry LSEntry) (string, error) {
	cmd := exec.CommandContext(ctx, "gem", "install", entry.Install.Package)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gem install failed for %s: %w", entry.Install.Package, err)
	}
	return exec.LookPath(entry.Command)
}

func (i *Installer) installDotnet(ctx context.Context, entry LSEntry) (string, error) {
	cmd := exec.CommandContext(ctx, "dotnet", "tool", "install", "-g", entry.Install.Package)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("dotnet tool install failed for %s: %w", entry.Install.Package, err)
	}
	return exec.LookPath(entry.Command)
}

func (i *Installer) installBinary(ctx context.Context, entry LSEntry) (string, error) {
	platform := PlatformKey()
	url, ok := entry.Install.URLs[platform]
	if !ok {
		return "", fmt.Errorf("no binary download URL for platform %q (language %q)", platform, entry.Language)
	}

	if err := os.MkdirAll(i.config.BinDir, 0755); err != nil {
		return "", fmt.Errorf("creating bin dir: %w", err)
	}

	// Download to temp file.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", entry.Command, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: HTTP %d for %s", resp.StatusCode, url)
	}

	tmpFile, err := os.CreateTemp(i.config.BinDir, entry.Command+"-*")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)
	if _, err := io.Copy(writer, resp.Body); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("writing download: %w", err)
	}
	_ = tmpFile.Close()

	// Verify checksum if available.
	if expected, hasChecksum := entry.Install.SHA256[platform]; hasChecksum {
		actual := hex.EncodeToString(hasher.Sum(nil))
		if actual != expected {
			return "", fmt.Errorf("checksum mismatch for %s: expected %s, got %s", entry.Command, expected, actual)
		}
	}

	// Move to final location and make executable.
	dest := filepath.Join(i.config.BinDir, entry.Command)
	if err := os.Rename(tmpFile.Name(), dest); err != nil {
		return "", fmt.Errorf("moving binary: %w", err)
	}
	if err := os.Chmod(dest, 0755); err != nil {
		return "", fmt.Errorf("chmod: %w", err)
	}

	return dest, nil
}
