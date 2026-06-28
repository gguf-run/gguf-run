package ggufrun

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gguf-run/finder/finder"
)

// IsValidGGUF checks whether the file at path starts with the GGUF magic
// bytes, confirming it is a valid GGUF model file.
func IsValidGGUF(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return false
	}
	return string(magic) == "GGUF"
}

// Download downloads a GGUF file from the given URL to the cache directory.
// The file name is derived from the URL. If a valid cached copy already exists
// it is returned without downloading. Returns the path to the downloaded file.
func Download(ctx context.Context, url, cacheDir string) (string, error) {
	name := filepath.Base(url)
	dest := filepath.Join(cacheDir, name)

	if fileExists(dest) {
		if IsValidGGUF(dest) {
			fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Using cached: %s\n", name)
			return dest, nil
		}
		fmt.Fprintf(os.Stderr, "\033[33m==>\033[0m Cached file corrupted, re-downloading: %s\n", name)
		os.Remove(dest)
	}

	fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Downloading %s ...\n", name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("download: creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("download: creating file: %w", err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(dest)
		return "", fmt.Errorf("download: incomplete: %w", err)
	}
	out.Close()

	if !IsValidGGUF(dest) {
		os.Remove(dest)
		return "", fmt.Errorf("download: file is not a valid GGUF model")
	}

	return dest, nil
}

// SearchAndDownload searches Hugging Face for a model matching the query,
// selects the best Q4_K_M quantized file, and downloads it. Returns the
// path to the downloaded file.
func SearchAndDownload(ctx context.Context, query, cacheDir string) (string, error) {
	results, err := Search(ctx, query, 20)
	if err != nil {
		return "", fmt.Errorf("search: %w", err)
	}

	if len(results) == 0 {
		return "", fmt.Errorf("no models found for: %s", query)
	}

	// prefer Q4_K_M quant
	for _, r := range results {
		for _, f := range r.Files {
			if strings.Contains(strings.ToLower(f), "q4_k_m") {
				fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Selected: %s\n", filepath.Base(f))
				return Download(ctx, f, cacheDir)
			}
		}
	}

	// fallback: first file of the first result
	url := results[0].Files[0]
	fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Selected: %s\n", filepath.Base(url))
	return Download(ctx, url, cacheDir)
}

// Ensure finder.Result is used (direct dependency)
var _ = finder.Search
