package llama

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

//go:embed libs/linux_amd64/* libs/linux_arm64/* libs/darwin_amd64/* libs/darwin_arm64/* libs/windows_amd64/*
var embeddedLibs embed.FS

var (
	extractOnce sync.Once
	extractErr  error
)

// Init initializes the llama.cpp library by extracting the embedded
// shared library for the current platform and loading it.
// This must be called before any other llama functions.
func Init() error {
	if err := LoadEmbedded(); err != nil {
		return err
	}
	return RegisterFunctions()
}

// MustInit initializes the library and panics on failure.
func MustInit() {
	if err := Init(); err != nil {
		panic("gguf-run: failed to initialize llama.cpp: " + err.Error())
	}
}

// extractEmbeddedLib extracts the embedded library for the current platform
// to the destination path.
func extractEmbeddedLib(destPath, goos, goarch string) error {
	extractOnce.Do(func() {
		// Determine the embedded directory name
		// Format: libs/<GOOS>_<GOARCH>/
		embedDir := fmt.Sprintf("libs/%s_%s", goos, goarch)

		// Check if the platform is embedded
		entries, err := fs.ReadDir(embeddedLibs, embedDir)
		if err != nil {
			extractErr = fmt.Errorf("platform %s not embedded: %w", embedDir, err)
			return
		}

		// Find the library file in the embedded directory
		var libFile string
		for _, e := range entries {
			if !e.IsDir() && (strings.HasSuffix(e.Name(), ".so") ||
				strings.HasSuffix(e.Name(), ".dylib") ||
				strings.HasSuffix(e.Name(), ".dll")) {
				libFile = e.Name()
				break
			}
		}

		if libFile == "" {
			extractErr = fmt.Errorf("no library file found in %s", embedDir)
			return
		}

		// Read the embedded library
		embedPath := filepath.ToSlash(filepath.Join(embedDir, libFile))
		data, err := fs.ReadFile(embeddedLibs, embedPath)
		if err != nil {
			extractErr = fmt.Errorf("read embedded lib: %w", err)
			return
		}

		// Ensure destination directory exists
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			extractErr = fmt.Errorf("mkdir dest: %w", err)
			return
		}

		// Write to destination
		extractErr = os.WriteFile(destPath, data, 0o755)
	})

	return extractErr
}

// Platform returns the current platform identifier used for embedding.
func Platform() string {
	return fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)
}

// EmbeddedPlatforms returns the list of platforms that have embedded libraries.
func EmbeddedPlatforms() []string {
	var platforms []string
	entries, err := fs.ReadDir(embeddedLibs, "libs")
	if err != nil {
		return platforms
	}
	for _, e := range entries {
		if e.IsDir() {
			platforms = append(platforms, e.Name())
		}
	}
	return platforms
}

// HasEmbeddedPlatform returns true if the given platform has an embedded library.
func HasEmbeddedPlatform(goos, goarch string) bool {
	platform := fmt.Sprintf("%s_%s", goos, goarch)
	for _, p := range EmbeddedPlatforms() {
		if p == platform {
			return true
		}
	}
	return false
}