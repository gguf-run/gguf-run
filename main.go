package main

/*
gguf-run — search, download, and run GGUF models with llama.cpp.

Subcommands:
  gguf-run run <model> [-p <prompt>] [--cache-dir <dir>] [-- <extra>]
  gguf-run server <model> [--addr <host:port>] [--cache-dir <dir>] [-- <extra>]
  gguf-run list [--cache-dir <dir>]
  gguf-run pull <model> [--cache-dir <dir>]
  gguf-run rm <model> [--cache-dir <dir>]
  gguf-run package <model> [--output <dir>] [--name <name>]
  gguf-run install llama.cpp    re-install / upgrade llama.cpp

Legacy (backward-compatible):
  gguf-run -q <query> [-m <path>] [-p <prompt>] [--cache-dir <dir>] [-- <extra>]

Installation:
  go install github.com/gguf-run/gguf-run@latest

As a library:
  import "github.com/gguf-run/gguf-run/ggufrun"
*/

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/gguf-run/gguf-run/ggufrun"
)

func defaultCacheDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "gguf")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	ensureLlamaCpp()

	subcmd := os.Args[1]

	switch subcmd {
	case "run":
		runCmd(os.Args[2:], "llama-cli", false)
	case "server":
		runCmd(os.Args[2:], "llama-server", true)
	case "list":
		listCmd(os.Args[2:])
	case "pull":
		pullCmd(os.Args[2:])
	case "rm":
		rmCmd(os.Args[2:])
	case "install":
		installCmd(os.Args[2:])
	case "package":
		packageCmd(os.Args[2:])
	default:
		if !strings.HasPrefix(subcmd, "-") {
			runCmd(os.Args[1:], "llama-cli", false)
			return
		}
		legacyMain()
	}
}

func printUsage() {
	fmt.Fprint(os.Stderr, `Usage:
  gguf-run run <model>          search, download, and run a model
  gguf-run server <model>       serve a model via HTTP API
  gguf-run list                 list cached models
  gguf-run pull <model>         download a model without running it
  gguf-run rm <model>           remove a cached model
  gguf-run package <model>      create a .cgp package referencing a model
  gguf-run install llama.cpp    re-install / upgrade llama.cpp

Flags:
  --cache-dir <dir>  model cache directory (default `+defaultCacheDir()+`)
  -p <prompt>        single-shot prompt (default: interactive)

Examples:
  gguf-run run tinyllama
  gguf-run run qwen2.5-1.5b -p "What is the capital of France?"
  gguf-run server phi -- --port 8080
  gguf-run list
  gguf-run pull smollm2-135m
  gguf-run rm smollm2-135m
  gguf-run package tinyllama
  gguf-run package ./model.gguf
`)
}

// ── run / server ─────────────────────────────────────────

func runCmd(args []string, binary string, isServer bool) {
	flags := flag.NewFlagSet(binary, flag.ExitOnError)
	prompt := flags.String("p", "", "single-shot prompt (default: interactive)")
	cacheDir := flags.String("cache-dir", defaultCacheDir(), "model cache directory")
	addr := flags.String("addr", ":8080", "server listen address (server only)")
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: gguf-run %s <model> [-p <prompt>] [--cache-dir <dir>] [-- <extra>]\n\nFlags:\n", binary)
		flags.PrintDefaults()
	}
	flags.Parse(args)

	model := flags.Arg(0)
	if model == "" {
		flags.Usage()
		os.Exit(1)
	}
	var extraArgs []string
	if flags.NArg() > 1 {
		extraArgs = flags.Args()[1:]
	}

	if err := os.MkdirAll(*cacheDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m Error creating cache directory: %v\n", err)
		os.Exit(1)
	}

	binPath := findBinary(binary)
	if binPath == "" {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m %s not found\n", binary)
		fmt.Fprint(os.Stderr, "Set LLAMACPP_DIR to your llama.cpp installation path.\n")
		os.Exit(1)
	}

	ctx := context.Background()
	modelFile, err := resolveModel(ctx, model, *cacheDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m %v\n", err)
		os.Exit(1)
	}

	if isServer {
		if err := ggufrun.RunServer(binPath, modelFile, *addr, extraArgs...); err != nil {
			fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m %v\n", err)
			os.Exit(1)
		}
	} else {
		var args []string
		if *prompt != "" {
			args = append(args, "-p", *prompt, "--single-turn")
		}
		args = append(args, extraArgs...)
		if err := ggufrun.Run(binPath, modelFile, args...); err != nil {
			fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m %v\n", err)
			os.Exit(1)
		}
	}
}

func findBinary(name string) string {
	switch name {
	case "llama-cli":
		return ggufrun.FindLlamaCli()
	case "llama-server":
		return ggufrun.FindLlamaServer()
	default:
		return ""
	}
}

// ── list ─────────────────────────────────────────────────

func listCmd(args []string) {
	flags := flag.NewFlagSet("list", flag.ExitOnError)
	cacheDir := flags.String("cache-dir", defaultCacheDir(), "model cache directory")
	flags.Parse(args)

	models, err := ggufrun.ListCached(*cacheDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m %v\n", err)
		os.Exit(1)
	}

	if len(models) == 0 {
		fmt.Fprintln(os.Stderr, "No cached models.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tSIZE\tMODIFIED")
	for _, m := range models {
		fmt.Fprintf(w, "%s\t%s\t%s\n", m.Name, formatSize(m.Size), m.ModTime.Format("Jan _2 15:04"))
	}
	w.Flush()
}

func formatSize(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	default:
		return fmt.Sprintf("%.1f KB", float64(b)/(1<<10))
	}
}

// ── pull ─────────────────────────────────────────────────

func pullCmd(args []string) {
	flags := flag.NewFlagSet("pull", flag.ExitOnError)
	cacheDir := flags.String("cache-dir", defaultCacheDir(), "model cache directory")
	flags.Parse(args)

	model := flags.Arg(0)
	if model == "" {
		fmt.Fprintln(os.Stderr, "Usage: gguf-run pull <model> [--cache-dir <dir>]")
		os.Exit(1)
	}

	if err := os.MkdirAll(*cacheDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m Error creating cache directory: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	modelFile, err := resolveModel(ctx, model, *cacheDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Model saved to %s\n", modelFile)
}

// ── rm ───────────────────────────────────────────────────

func rmCmd(args []string) {
	flags := flag.NewFlagSet("rm", flag.ExitOnError)
	cacheDir := flags.String("cache-dir", defaultCacheDir(), "model cache directory")
	flags.Parse(args)

	model := flags.Arg(0)
	if model == "" {
		fmt.Fprintln(os.Stderr, "Usage: gguf-run rm <model> [--cache-dir <dir>]")
		os.Exit(1)
	}

	if err := ggufrun.RemoveCached(model, *cacheDir); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Removed %s\n", model)
}

// ── package ──────────────────────────────────────────────

func packageCmd(args []string) {
	flags := flag.NewFlagSet("package", flag.ExitOnError)
	outputDir := flags.String("output", ".", "output directory for the .cgp file")
	pkgName := flags.String("name", "", "package name (default: derived from model filename)")
	flags.Parse(args)

	model := flags.Arg(0)
	if model == "" {
		fmt.Fprintln(os.Stderr, "Usage: gguf-run package <model> [--output <dir>] [--name <name>]")
		fmt.Fprintln(os.Stderr, "  <model> can be a search query, URL, or local .gguf file")
		os.Exit(1)
	}

	ctx := context.Background()

	ref, name, err := resolveGgufRef(ctx, model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m %v\n", err)
		os.Exit(1)
	}

	if *pkgName != "" {
		name = *pkgName
	}

	outPath, err := ggufrun.BuildCgp(name, ref, *outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Package created: %s\n", outPath)
	fmt.Fprintf(os.Stderr, "\033[33m==>\033[0m Install with: cpm install %s\n", filepath.Base(outPath))
}

func resolveGgufRef(ctx context.Context, model string) (*ggufrun.GgufRef, string, error) {
	if strings.HasPrefix(model, "http://") || strings.HasPrefix(model, "https://") {
		return resolveURLRef(ctx, model)
	}
	if info, err := os.Stat(model); err == nil && !info.IsDir() {
		return resolveLocalRef(model, info.Size())
	}
	return resolveSearchRef(ctx, model)
}

func resolveURLRef(ctx context.Context, url string) (*ggufrun.GgufRef, string, error) {
	filename := filepath.Base(url)
	name := strings.TrimSuffix(filename, ".gguf")
	name = strings.TrimSuffix(name, ".GGUF")

	ref := &ggufrun.GgufRef{
		Source:   "huggingface",
		URL:      url,
		Filename: filename,
	}

	if resp, err := http.Head(url); err == nil && resp.ContentLength > 0 {
		ref.SizeBytes = resp.ContentLength
	}

	return ref, name, nil
}

func resolveLocalRef(path string, size int64) (*ggufrun.GgufRef, string, error) {
	filename := filepath.Base(path)
	name := strings.TrimSuffix(filename, ".gguf")
	name = strings.TrimSuffix(name, ".GGUF")

	ref := &ggufrun.GgufRef{
		Source:    "local",
		URL:       "",
		Filename:  filename,
		SizeBytes: size,
	}

	f, err := os.Open(path)
	if err == nil {
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, f); err == nil {
			ref.SHA256 = fmt.Sprintf("%x", h.Sum(nil))
		}
	}

	return ref, name, nil
}

func resolveSearchRef(ctx context.Context, query string) (*ggufrun.GgufRef, string, error) {
	fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Searching: %s\n", query)

	res, err := ggufrun.SearchBest(ctx, query, 20)
	if err != nil {
		return nil, "", err
	}

	quant := extractQuant(res.Filename)
	name := strings.TrimSuffix(res.Filename, ".gguf")
	name = strings.TrimSuffix(name, ".GGUF")

	ref := &ggufrun.GgufRef{
		Source:   "huggingface",
		ModelID:  res.ModelID,
		URL:      res.URL,
		Filename: res.Filename,
		Quant:    quant,
	}

	if resp, err := http.Head(res.URL); err == nil && resp.ContentLength > 0 {
		ref.SizeBytes = resp.ContentLength
	}

	fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Selected: %s\n", filepath.Base(res.URL))
	return ref, name, nil
}

func extractQuant(filename string) string {
	parts := strings.Split(strings.ToUpper(filename), ".")
	for _, p := range parts {
		if strings.Contains(p, "Q") && (strings.Contains(p, "K") || strings.Contains(p, "_")) {
			return p
		}
	}
	return ""
}

// ── first-launch dependency check ────────────────────────

func ensureLlamaCpp() {
	if ggufrun.FindLlamaCli() != "" {
		return
	}
	fmt.Fprint(os.Stderr, "\033[33m==>\033[0m llama-cli not found — installing llama.cpp...\n")
	if err := ggufrun.InstallLlamaCpp(); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m %v\n", err)
		fmt.Fprint(os.Stderr, "Set LLAMACPP_DIR to your llama.cpp installation path and try again.\n")
		os.Exit(1)
	}
}

// ── install (re-install / upgrade) ────────────────────────

func installCmd(args []string) {
	what := "llama.cpp"
	if len(args) > 0 {
		what = args[0]
	}
	if what != "llama.cpp" {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m unknown: %s\n", what)
		fmt.Fprint(os.Stderr, "Usage: gguf-run install llama.cpp\n")
		os.Exit(1)
	}
	if err := ggufrun.InstallLlamaCpp(); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m %v\n", err)
		os.Exit(1)
	}
}

// ── model resolution ─────────────────────────────────────

func resolveModel(ctx context.Context, model, cacheDir string) (string, error) {
	if strings.HasPrefix(model, "http://") || strings.HasPrefix(model, "https://") {
		return ggufrun.Download(ctx, model, cacheDir)
	}
	if info, err := os.Stat(model); err == nil {
		if info.IsDir() {
			if isCgpDir(model) {
				return resolveCgpDir(ctx, model, cacheDir)
			}
			return "", fmt.Errorf("is a directory: %s", model)
		}
		if strings.HasSuffix(model, ".cgp") {
			return resolveCgpFile(ctx, model, cacheDir)
		}
		return model, nil
	}
	fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Searching: %s\n", model)
	return ggufrun.SearchAndDownload(ctx, model, cacheDir)
}

func isCgpDir(path string) bool {
	_, err := os.Stat(filepath.Join(path, "cognitive.json"))
	return err == nil
}

func resolveCgpFile(ctx context.Context, path, cacheDir string) (string, error) {
	ref, err := ggufrun.ReadGgufRef(path)
	if err != nil {
		return "", fmt.Errorf("read .cgp: %w", err)
	}
	fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Using gguf reference from %s\n", filepath.Base(path))
	return ggufrun.Download(ctx, ref.URL, cacheDir)
}

func resolveCgpDir(ctx context.Context, path, cacheDir string) (string, error) {
	ref, err := ggufrun.ReadGgufRef(path)
	if err != nil {
		return "", fmt.Errorf("read cgp directory: %w", err)
	}
	fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Using gguf reference from %s\n", path)
	return ggufrun.Download(ctx, ref.URL, cacheDir)
}

// ── legacy entry point (backward compat) ─────────────────

func legacyMain() {
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
		flag.Usage()
		os.Exit(1)
	}

	binPath := findBinary("llama-cli")
	if binPath == "" {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m llama-cli not found\n")
		fmt.Fprint(os.Stderr, "Set LLAMACPP_DIR to your llama.cpp installation path.\n")
		os.Exit(1)
	}

	if err := os.MkdirAll(*cacheDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m Error creating cache directory: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	var modelFile string
	var err error

	if *modelPath != "" {
		modelFile, err = resolvePathModel(ctx, *modelPath, *cacheDir)
	} else {
		fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Searching: %s\n", *searchQuery)
		modelFile, err = ggufrun.SearchAndDownload(ctx, *searchQuery, *cacheDir)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m %v\n", err)
		os.Exit(1)
	}

	var args []string
	if *prompt != "" {
		args = []string{"-p", *prompt, "--single-turn"}
	}
	args = append(args, flag.Args()...)

	if err := ggufrun.Run(binPath, modelFile, args...); err != nil {
		fmt.Fprintf(os.Stderr, "\n\033[31m==>\033[0m %v\n", err)
		os.Exit(1)
	}
}

func resolvePathModel(ctx context.Context, modelPath, cacheDir string) (string, error) {
	if strings.HasPrefix(modelPath, "http://") || strings.HasPrefix(modelPath, "https://") {
		return ggufrun.Download(ctx, modelPath, cacheDir)
	}
	if _, err := os.Stat(modelPath); err != nil {
		return "", fmt.Errorf("file not found: %s", modelPath)
	}
	return modelPath, nil
}
