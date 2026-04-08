package langregistry

import (
	"context"
	"log/slog"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallerResolvePATH(t *testing.T) {
	// "go" should always be in PATH on a dev machine running Go tests.
	installer := NewInstaller(InstallerConfig{AutoInstall: false}, slog.Default())
	entry := LSEntry{
		Language: "go",
		Command:  "go",
		Args:     []string{"version"},
	}
	cmd, args, err := installer.Resolve(context.Background(), entry)
	require.NoError(t, err)
	assert.NotEmpty(t, cmd)
	assert.Equal(t, []string{"version"}, args)
}

func TestInstallerResolveAutoInstallDisabledMissing(t *testing.T) {
	installer := NewInstaller(InstallerConfig{AutoInstall: false}, slog.Default())
	entry := LSEntry{
		Language: "fake_lang",
		Command:  "nonexistent-binary-xyz-12345",
		Install: &InstallInfo{
			Type:    "npm",
			Package: "nonexistent-pkg",
		},
	}
	_, _, err := installer.Resolve(context.Background(), entry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "npm install -g nonexistent-pkg")
}

func TestInstallerResolveNoInstallInfo(t *testing.T) {
	installer := NewInstaller(InstallerConfig{AutoInstall: true}, slog.Default())
	entry := LSEntry{
		Language: "fake_lang",
		Command:  "nonexistent-binary-xyz-12345",
		Install:  nil,
	}
	_, _, err := installer.Resolve(context.Background(), entry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Install \"nonexistent-binary-xyz-12345\" manually")
}

func TestInstallerInstallHintVariants(t *testing.T) {
	tests := []struct {
		name     string
		install  *InstallInfo
		command  string
		contains string
	}{
		{"npm", &InstallInfo{Type: "npm", Package: "foo", Version: "1.0"}, "foo", "npm install -g foo@1.0"},
		{"pip", &InstallInfo{Type: "pip", Package: "bar"}, "bar", "pip install bar"},
		{"cargo", &InstallInfo{Type: "cargo", Package: "baz", Version: "2.0"}, "baz", "cargo install baz --version 2.0"},
		{"gem", &InstallInfo{Type: "gem", Package: "qux"}, "qux", "gem install qux"},
		{"dotnet", &InstallInfo{Type: "dotnet", Package: "quux"}, "quux", "dotnet tool install -g quux"},
		{"binary", &InstallInfo{Type: "binary", Package: "corge"}, "corge", "Download corge"},
		{"nil", nil, "my-ls", "Install \"my-ls\" manually"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := LSEntry{Command: tt.command, Install: tt.install}
			assert.Contains(t, e.InstallHint(), tt.contains)
		})
	}
}

func TestPlatformKey(t *testing.T) {
	key := PlatformKey()
	assert.Contains(t, key, runtime.GOOS)
	assert.Contains(t, key, runtime.GOARCH)
	assert.Equal(t, runtime.GOOS+"-"+runtime.GOARCH, key)
}
