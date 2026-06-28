package ggufrun

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Run(llamaCli, modelPath string, extraArgs ...string) error {
	args := []string{"-m", modelPath}
	args = append(args, extraArgs...)

	name := filepath.Base(llamaCli)
	fmt.Fprintf(os.Stderr, "\n\033[32m==>\033[0m Running: %s %s\n\n", name, joinArgs(args))

	cmd := exec.Command(llamaCli, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s exited: %w", name, err)
	}
	return nil
}

// RunServer starts llama-server with the given model. It listens on addr
// (default :8080) and passes any extraArgs to the binary.
func RunServer(llamaServer, modelPath, addr string, extraArgs ...string) error {
	args := []string{"-m", modelPath, "--host", "--port", addr}
	args = append(args, extraArgs...)

	name := filepath.Base(llamaServer)
	fmt.Fprintf(os.Stderr, "\n\033[32m==>\033[0m Running: %s %s\n\n", name, joinArgs(args))

	cmd := exec.Command(llamaServer, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s exited: %w", name, err)
	}
	return nil
}

func joinArgs(args []string) string {
	s := ""
	for _, a := range args {
		if s != "" {
			s += " "
		}
		if strings.Contains(a, " ") {
			s += `"` + a + `"`
		} else {
			s += a
		}
	}
	return s
}
