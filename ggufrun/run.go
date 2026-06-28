package ggufrun

import (
	"fmt"
	"os"
	"os/exec"
)

// Run executes llama-cli with the given model and optional extra arguments.
// It connects stdin, stdout, and stderr directly to the terminal.
func Run(llamaCli, modelPath string, extraArgs ...string) error {
	args := []string{"-m", modelPath}
	args = append(args, extraArgs...)

	fmt.Fprintf(os.Stderr, "\n\033[32m==>\033[0m Running: llama-cli %s\n\n", formatArgs(args))

	cmd := exec.Command(llamaCli, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("llama-cli exited: %w", err)
	}
	return nil
}

func formatArgs(args []string) string {
	s := ""
	for _, a := range args {
		if s != "" {
			s += " "
		}
		if containsSpace(a) {
			s += `"` + a + `"`
		} else {
			s += a
		}
	}
	return s
}

func containsSpace(s string) bool {
	for _, c := range s {
		if c == ' ' {
			return true
		}
	}
	return false
}
