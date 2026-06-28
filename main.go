package main

/*
gguf-run — search, download, and run GGUF models with llama.cpp in one command.

Searches Hugging Face Hub for a model (preferring Q4_K_M quant), downloads
it if not cached, and launches llama-cli in interactive or single-prompt mode.

Installation:
  go install github.com/gguf-run/gguf-run@latest

Usage:
  gguf-run -q <query> [-p <prompt>] [-m <path>] [--cache-dir <dir>] [-- <llama-cli args>]

Examples:
  gguf-run -q tinyllama
  gguf-run -q qwen2.5-1.5b -p "What is the capital of France?"
  gguf-run -m ~/models/my-model.q4_k_m.gguf
  gguf-run -q phi -- --temp 0.8 --ctx-size 4096 -ngl 999
*/

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ── data types (mirrors finder JSON output) ─────────────

type FinderOutput struct {
	Query   string         `json:"query"`
	Limit   int            `json:"limit"`
	Results []FinderResult `json:"results"`
}

type FinderResult struct {
	ModelID   string   `json:"modelId"`
	ModelName string   `json:"modelName"`
	Files     []string `json:"files"`
}

// ── helpers ───────────────────────────────────────────────

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func defaultCacheDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "gguf")
}

func isValidGGUF(path string) bool {
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

// ── llama-cli discovery ──────────────────────────────────

func findLlamaCli() string {
	if dir := os.Getenv("LLAMACPP_DIR"); dir != "" {
		p := filepath.Join(dir, "bin", "llama-cli")
		if fileExists(p) {
			return p
		}
	}
	if p, err := exec.LookPath("llama-cli"); err == nil {
		return p
	}
	for _, p := range []string{
		"/usr/local/opt/llama.cpp/bin/llama-cli",
		"/opt/homebrew/opt/llama.cpp/bin/llama-cli",
	} {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

func installLlamaCpp() string {
	if _, err := exec.LookPath("brew"); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m Homebrew not found. Install llama.cpp manually:\n")
		fmt.Fprintf(os.Stderr, "  brew install llama.cpp\n")
		return ""
	}

	fmt.Fprintf(os.Stderr, "\033[33m==>\033[0m llama-cli not found. Install llama.cpp now? [Y/n]: ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return ""
	}
	answer := strings.TrimSpace(scanner.Text())
	if answer == "n" || answer == "N" {
		return ""
	}

	fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Installing llama.cpp...\n")
	cmd := exec.Command("brew", "install", "llama.cpp")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m Installation failed: %v\n", err)
		return ""
	}

	return findLlamaCli()
}

// ── finder discovery ─────────────────────────────────────

func findFinder() (string, []string) {
	if p, err := exec.LookPath("finder"); err == nil {
		return p, nil
	}
	if fileExists("./finder") {
		return "./finder", nil
	}
	if fileExists("./main.go") {
		return "go", []string{"run", "./main.go"}
	}
	return "", nil
}

// ── model resolution ─────────────────────────────────────

func downloadModel(url, cacheDir string) (string, error) {
	name := filepath.Base(url)
	dest := filepath.Join(cacheDir, name)

	if fileExists(dest) {
		if isValidGGUF(dest) {
			fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Using cached: %s\n", name)
			return dest, nil
		}
		fmt.Fprintf(os.Stderr, "\033[33m==>\033[0m Cached file corrupted, re-downloading: %s\n", name)
		os.Remove(dest)
	}

	fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Downloading %s ...\n", name)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("cannot create file: %w", err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(dest)
		return "", fmt.Errorf("download incomplete: %w", err)
	}
	out.Close()

	if !isValidGGUF(dest) {
		os.Remove(dest)
		return "", fmt.Errorf("downloaded file is not a valid GGUF model")
	}

	return dest, nil
}

func resolvePathModel(modelPath, cacheDir string) (string, error) {
	if strings.HasPrefix(modelPath, "http://") || strings.HasPrefix(modelPath, "https://") {
		return downloadModel(modelPath, cacheDir)
	}
	if !fileExists(modelPath) {
		return "", fmt.Errorf("file not found: %s", modelPath)
	}
	return modelPath, nil
}

func resolveQueryModel(query, cacheDir string) (string, error) {
	fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Searching: %s\n", query)

	cmdName, cmdArgs := findFinder()
	if cmdName == "" {
		return "", fmt.Errorf("finder not found. Install it:\n  go install github.com/gguf-run/finder@latest")
	}

	cmdArgs = append(cmdArgs, "-q", query, "-limit", "20", "-json")
	cmd := exec.Command(cmdName, cmdArgs...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	var fo FinderOutput
	if err := json.Unmarshal(output, &fo); err != nil {
		return "", fmt.Errorf("failed to parse search results: %w", err)
	}

	if len(fo.Results) == 0 {
		return "", fmt.Errorf("no models found for: %s", query)
	}

	// prefer Q4_K_M quant
	for _, r := range fo.Results {
		for _, f := range r.Files {
			if strings.Contains(strings.ToLower(f), "q4_k_m") {
				fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Selected: %s\n", filepath.Base(f))
				return downloadModel(f, cacheDir)
			}
		}
	}

	// fallback: first file of the first result
	url := fo.Results[0].Files[0]
	fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Selected: %s\n", filepath.Base(url))
	return downloadModel(url, cacheDir)
}

func resolveModel(searchQuery, modelPath, cacheDir string) (string, error) {
	if modelPath != "" {
		return resolvePathModel(modelPath, cacheDir)
	}
	return resolveQueryModel(searchQuery, cacheDir)
}

// ── main ─────────────────────────────────────────────────

func main() {
	searchQuery := flag.String("q", "", "Hugging Face search query")
	modelPath := flag.String("m", "", "model file path or download URL")
	prompt := flag.String("p", "", "single-shot prompt (default: interactive chat)")
	cacheDir := flag.String("cache-dir", defaultCacheDir(), "model cache directory")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: gguf-run -q <query> [-p <prompt>] [-m <path>] [--cache-dir <dir>] [-- <llama-cli args>]\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *searchQuery == "" && *modelPath == "" {
		fmt.Fprintf(os.Stderr, "Usage: gguf-run -q <query> [-p <prompt>] [-m <path>] [--cache-dir <dir>] [-- <llama-cli args>]\n\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	llamaCli := findLlamaCli()
	if llamaCli == "" {
		llamaCli = installLlamaCpp()
		if llamaCli == "" {
			os.Exit(1)
		}
	}

	if err := os.MkdirAll(*cacheDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m Error creating cache directory: %v\n", err)
		os.Exit(1)
	}

	modelFile, err := resolveModel(*searchQuery, *modelPath, *cacheDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m %v\n", err)
		os.Exit(1)
	}

	args := []string{"-m", modelFile}
	if *prompt != "" {
		args = append(args, "-p", *prompt, "--single-turn")
	}
	args = append(args, flag.Args()...)

	fmt.Fprintf(os.Stderr, "\n\033[32m==>\033[0m Running: llama-cli %s\n\n", strings.Join(args, " "))

	cmd := exec.Command(llamaCli, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "\n\033[31m==>\033[0m llama-cli exited: %v\n", err)
		os.Exit(1)
	}
}
