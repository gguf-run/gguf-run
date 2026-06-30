//go:build !windows

package llama

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/ebitengine/purego"
)

var (
	libHandle uintptr
	libPath   string
	libMutex  sync.Mutex
	isLoaded  bool
)

// LoadEmbedded extracts the embedded library for the current platform
// to a cache directory and loads it via dlopen.
// If no embedded library is found, it falls back to system paths.
func LoadEmbedded() error {
	libMutex.Lock()
	defer libMutex.Unlock()

	if isLoaded {
		return nil
	}

	// Determine platform-specific library name
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	libName := libraryName(goos, goarch)
	if libName == "" {
		return fmt.Errorf("unsupported platform: %s/%s", goos, goarch)
	}

	// Try embedded library first
	cacheDirPath, err := cacheDir()
	if err == nil {
		destPath := filepath.Join(cacheDirPath, libName)
		if err := extractEmbeddedLib(destPath, goos, goarch); err == nil {
			handle, err := purego.Dlopen(destPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
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
		"/usr/local/lib/" + libName,
		"/usr/lib/" + libName,
		filepath.Join(os.Getenv("HOME"), ".local", "lib", libName),
	}
	if ld := os.Getenv("LD_LIBRARY_PATH"); ld != "" {
		for _, dir := range strings.Split(ld, ":") {
			systemPaths = append(systemPaths, filepath.Join(dir, libName))
		}
	}

	for _, path := range systemPaths {
		if _, err := os.Stat(path); err == nil {
			handle, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
			if err == nil {
				libHandle = handle
				libPath = path
				isLoaded = true
				return nil
			}
		}
	}

	// Search current directory
	if _, err := os.Stat(libName); err == nil {
		handle, err := purego.Dlopen(libName, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err == nil {
			libHandle = handle
			libPath = libName
			isLoaded = true
			return nil
		}
	}

	// Auto-download from GitHub releases
	var dlErr error
	cacheDirPath, err = cacheDir()
	canDownload, reason := autoDownloadSupported()
	if canDownload && err == nil {
		destPath, deps, aerr := autoDownloadLib(cacheDirPath)
		if aerr == nil {
			// Load dependencies first so symbols are available
			for _, dep := range deps {
				_, depErr := purego.Dlopen(dep, purego.RTLD_NOW|purego.RTLD_GLOBAL)
				if depErr != nil {
					fmt.Fprintf(os.Stderr, "\033[33m==>\033[0m Warning: failed to load dependency %s: %v\n", dep, depErr)
				}
			}
			handle, loadErr := purego.Dlopen(destPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
			if loadErr == nil {
				libHandle = handle
				libPath = destPath
				isLoaded = true
				return nil
			}
			dlErr = loadErr
		} else {
			dlErr = aerr
		}
	}

	msg := fmt.Sprintf("llama.cpp library (%s) not found", libName)
	switch {
	case !canDownload:
		msg += "\n  " + reason + ". Install via apk: apk add llama.cpp-libs"
	case dlErr != nil:
		msg += fmt.Sprintf("\n  auto-download succeeded but library failed to load: %v", dlErr)
	default:
		msg += "\n  No embedded library found for this platform"
	}
	msg += "\n  Alternatively, run 'make populate-libs' from the source tree to embed the library, or install llama.cpp system-wide"
	return fmt.Errorf("%s", msg)
}

// Handle returns the loaded library handle.
func Handle() uintptr {
	libMutex.Lock()
	defer libMutex.Unlock()
	return libHandle
}

// IsLoaded returns whether the library is loaded.
func IsLoaded() bool {
	libMutex.Lock()
	defer libMutex.Unlock()
	return isLoaded
}

// libraryName returns the platform-specific library filename.
func libraryName(goos, goarch string) string {
	switch goos {
	case "darwin":
		return "libllama.dylib"
	case "linux", "freebsd", "openbsd", "netbsd":
		return "libllama.so"
	default:
		return ""
	}
}

// cacheDir returns the cache directory for extracted libraries.
func cacheDir() (string, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".cache")
	}
	dir := filepath.Join(base, "gguf", "lib")
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

	purego.Dlclose(libHandle)
	libHandle = 0
	libPath = ""
	isLoaded = false
	return nil
}