package llama

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

// isMusl returns true if the system uses musl libc instead of glibc.
// On such systems, glibc-linked pre-built binaries won't load.
func isMusl() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	muslPaths := []string{
		"/lib/ld-musl-x86_64.so.1",
		"/lib/ld-musl-aarch64.so.1",
		"/lib/ld-musl-armhf.so.1",
	}
	for _, p := range muslPaths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// autoDownloadSupported returns true if pre-built libraries can be downloaded
// and loaded on this system. Pre-built libs link against glibc and won't work
// on musl-based distros (Alpine, etc.).
func autoDownloadSupported() (bool, string) {
	if isMusl() {
		return false, "pre-built libraries link against glibc and are incompatible with musl libc"
	}
	return true, ""
}

// llama.cpp release build to use for auto-download.
const llamaCppBuild = "b6099"

// llamaCppReleaseURL is the base URL for llama.cpp pre-built releases.
const llamaCppReleaseURL = "https://github.com/ggml-org/llama.cpp/releases/download/" + llamaCppBuild + "/"

// platformAsset returns the zip asset name for the current platform.
func platformAsset() string {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "linux/amd64":
		return "llama-" + llamaCppBuild + "-bin-ubuntu-x64.zip"
	case "linux/arm64":
		return "llama-" + llamaCppBuild + "-bin-ubuntu-arm64.zip"
	case "darwin/amd64":
		return "llama-" + llamaCppBuild + "-bin-macos-x64.zip"
	case "darwin/arm64":
		return "llama-" + llamaCppBuild + "-bin-macos-arm64.zip"
	case "windows/amd64":
		return "llama-" + llamaCppBuild + "-bin-win-cpu-x64.zip"
	default:
		return ""
	}
}

// libraryFileName returns the expected shared library filename after extraction.
func libraryFileName() string {
	switch runtime.GOOS {
	case "windows":
		return "llama.dll"
	case "darwin":
		return "libllama.dylib"
	default:
		return "libllama.so"
	}
}

// libDeps returns the dependency .so files that must be loaded before libllama.
func libDeps() []string {
	switch runtime.GOOS {
	case "linux":
		return []string{
			"libggml.so",
			"libggml-base.so",
			"libggml-cpu-x64.so",
		}
	case "darwin":
		return []string{
			"libggml.dylib",
			"libggml-base.dylib",
		}
	default:
		return nil
	}
}

// autoDownloadLib downloads the pre-built llama.cpp shared library for the
// current platform, extracts it to the cache directory, and returns the path
// to the extracted library file and its dependencies.
func autoDownloadLib(cacheDir string) (libPath string, depPaths []string, err error) {
	asset := platformAsset()
	if asset == "" {
		return "", nil, fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	libName := libraryFileName()
	destPath := filepath.Join(cacheDir, libName)

	// Check if already downloaded
	if _, err := os.Stat(destPath); err == nil {
		return destPath, checkDeps(cacheDir), nil
	}

	url := llamaCppReleaseURL + asset
	fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Downloading llama.cpp shared library (%s) ...\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return "", nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}

	// Buffer the full zip (need seekable reader)
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("read response: %w", err)
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", nil, fmt.Errorf("mkdir cache: %w", err)
	}

	// Extract from zip
	zr, err := zip.NewReader(readerAtBytes(data), int64(len(data)))
	if err != nil {
		return "", nil, fmt.Errorf("open zip: %w", err)
	}

	// Collect all .so files we need to extract
	wanted := map[string]bool{libName: true}
	for _, dep := range libDeps() {
		wanted[filepath.Base(dep)] = true
	}

	found := map[string]bool{}
	for _, f := range zr.File {
		base := filepath.Base(f.Name)
		if !wanted[base] {
			continue
		}

		dest := filepath.Join(cacheDir, base)
		if err := extractZipEntry(f, dest); err != nil {
			return "", nil, fmt.Errorf("extract %s: %w", base, err)
		}
		found[base] = true
	}

	// Check all wanted files were found
	for name := range wanted {
		if !found[name] {
			return "", nil, fmt.Errorf("%s not found in archive", name)
		}
	}

	depPaths = checkDeps(cacheDir)
	return destPath, depPaths, nil
}

func extractZipEntry(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	outFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, rc)
	return err
}

func checkDeps(cacheDir string) []string {
	var deps []string
	for _, dep := range libDeps() {
		path := filepath.Join(cacheDir, filepath.Base(dep))
		if _, err := os.Stat(path); err == nil {
			deps = append(deps, path)
		}
	}
	return deps
}

// readerAtBytes implements io.ReaderAt for a byte slice.
type readerAtBytes []byte

func (b readerAtBytes) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(b)) {
		return 0, io.EOF
	}
	n := copy(p, b[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
