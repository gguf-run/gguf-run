package ggufrun

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FindLlamaCli returns the path to the llama-cli binary.
// It checks (in order): LLAMACPP_DIR env var, PATH, and common Homebrew locations.
// Returns empty string if not found.
func FindLlamaCli() string {
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

// InstallLlamaCpp prompts the user to install llama.cpp via Homebrew
// and performs the installation. Returns an error if Homebrew is not
// found or installation fails.
func InstallLlamaCpp() error {
	if _, err := exec.LookPath("brew"); err != nil {
		return fmt.Errorf("Homebrew not found; install llama.cpp manually: brew install llama.cpp")
	}

	fmt.Fprint(os.Stderr, "\033[33m==>\033[0m llama-cli not found. Install llama.cpp now? [Y/n]: ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return fmt.Errorf("cancelled")
	}
	answer := strings.TrimSpace(scanner.Text())
	if answer == "n" || answer == "N" {
		return fmt.Errorf("cancelled")
	}

	fmt.Fprint(os.Stderr, "\033[32m==>\033[0m Installing llama.cpp...\n")
	cmd := exec.Command("brew", "install", "llama.cpp")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
