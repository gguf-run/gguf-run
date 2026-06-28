package main

/*
gguf-run — search, download, and run GGUF models with llama.cpp in one command.

Searches Hugging Face Hub for a model (preferring Q4_K_M quant), downloads
it if not cached, and launches llama-cli in interactive or single-prompt mode.

Installation:
  go install github.com/gguf-run/gguf-run@latest

As a library:
  import "github.com/gguf-run/gguf-run/ggufrun"

Usage:
  gguf-run -q <query> [-p <prompt>] [-m <path>] [--cache-dir <dir>] [-- <llama-cli args>]

Examples:
  gguf-run -q tinyllama
  gguf-run -q qwen2.5-1.5b -p "What is the capital of France?"
  gguf-run -m ~/models/my-model.q4_k_m.gguf
  gguf-run -q phi -- --temp 0.8 --ctx-size 4096 -ngl 999
*/

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	llamaCli := ggufrun.FindLlamaCli()
	if llamaCli == "" {
		if err := ggufrun.InstallLlamaCpp(); err != nil {
			fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m %v\n", err)
			os.Exit(1)
		}
		llamaCli = ggufrun.FindLlamaCli()
		if llamaCli == "" {
			fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m llama-cli still not found after install\n")
			os.Exit(1)
		}
	}

	if err := os.MkdirAll(*cacheDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m Error creating cache directory: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	modelFile, err := resolveModel(ctx, *searchQuery, *modelPath, *cacheDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m==>\033[0m %v\n", err)
		os.Exit(1)
	}

	var args []string
	if *prompt != "" {
		args = []string{"-p", *prompt, "--single-turn"}
	}
	args = append(args, flag.Args()...)

	if err := ggufrun.Run(llamaCli, modelFile, args...); err != nil {
		fmt.Fprintf(os.Stderr, "\n\033[31m==>\033[0m %v\n", err)
		os.Exit(1)
	}
}

func resolveModel(ctx context.Context, searchQuery, modelPath, cacheDir string) (string, error) {
	if modelPath != "" {
		return resolvePathModel(ctx, modelPath, cacheDir)
	}
	fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Searching: %s\n", searchQuery)
	return ggufrun.SearchAndDownload(ctx, searchQuery, cacheDir)
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
