//go:build windows

package llama

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"golang.org/x/sys/windows"
)

var (
	libHandle windows.Handle
	libPath   string
	libMutex  sync.Mutex
	isLoaded  bool
)

// LoadEmbedded extracts the embedded library for the current platform
// to a cache directory and loads it via LoadLibrary.
// If no embedded library is found, it falls back to system paths.
func LoadEmbedded() error {
	libMutex.Lock()
	defer libMutex.Unlock()

	if isLoaded {
		return nil
	}

	// Try embedded library first
	cacheDirPath, err := cacheDir()
	if err == nil {
		destPath := filepath.Join(cacheDirPath, "llama.dll")
		if err := extractEmbeddedLib(destPath, runtime.GOOS, runtime.GOARCH); err == nil {
			handle, err := windows.LoadLibrary(destPath)
			if err == nil {
				libHandle = handle
				libPath = destPath
				isLoaded = true
				return nil
			}
		}
	}

	// Fallback: search system paths
	systemPaths := []string{
		"llama.dll",
		filepath.Join(os.Getenv("PATH"), "llama.dll"),
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		systemPaths = append(systemPaths, filepath.Join(local, "gguf-run", "lib", "llama.dll"))
	}

	for _, path := range systemPaths {
		handle, err := windows.LoadLibrary(path)
		if err == nil {
			libHandle = handle
			libPath = path
			isLoaded = true
			return nil
		}
	}

	// Auto-download from GitHub releases
	cacheDirPath, err := cacheDir()
	if err == nil {
		destPath, deps, err := autoDownloadLib(cacheDirPath)
		if err == nil {
			// Load dependencies first
			for _, dep := range deps {
				windows.LoadLibrary(dep)
			}
			handle, err := windows.LoadLibrary(destPath)
			if err == nil {
				libHandle = handle
				libPath = destPath
				isLoaded = true
				return nil
			}
		}
	}

	return fmt.Errorf("llama.dll not found: install llama.cpp or run 'make populate-libs' to embed it")
}

// Handle returns the loaded library handle.
func Handle() uintptr {
	libMutex.Lock()
	defer libMutex.Unlock()
	return uintptr(libHandle)
}

// IsLoaded returns whether the library is loaded.
func IsLoaded() bool {
	libMutex.Lock()
	defer libMutex.Unlock()
	return isLoaded
}

// cacheDir returns the cache directory for extracted libraries.
func cacheDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, "AppData", "Local")
	}
	dir := filepath.Join(base, "gguf-run", "lib")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// Close unloads the library.
func Close() error {
	libMutex.Lock()
	defer libMutex.Unlock()

	if !isLoaded {
		return nil
	}

	windows.FreeLibrary(libHandle)
	libHandle = 0
	libPath = ""
	isLoaded = false
	return nil
}