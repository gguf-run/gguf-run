package ggufrun

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// FindLlamaCli returns the path to the llama-cli binary.
// Checks (in order): LLAMACPP_DIR env var, PATH, and common platform locations.
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

	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".local", "bin", "llama-cli"),
		"/usr/local/bin/llama-cli",
	}

	if runtime.GOOS == "darwin" {
		candidates = append(candidates,
			"/usr/local/opt/llama.cpp/bin/llama-cli",
			"/opt/homebrew/opt/llama.cpp/bin/llama-cli",
		)
	}

	for _, p := range candidates {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

// InstallLlamaCpp installs llama.cpp using the best method for the current
// platform: Homebrew on macOS, system package manager on Linux, vcpkg on
// Windows, with a universal cmake source build as fallback.
func InstallLlamaCpp() error {
	fmt.Fprint(os.Stderr, "\033[33m==>\033[0m llama-cli not found. Install llama.cpp now? [Y/n]: ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return fmt.Errorf("cancelled")
	}
	answer := strings.TrimSpace(scanner.Text())
	if answer == "n" || answer == "N" {
		return fmt.Errorf("cancelled")
	}

	switch runtime.GOOS {
	case "darwin":
		return installOnDarwin()
	case "linux":
		return installOnLinux()
	case "windows":
		return installOnWindows()
	default:
		return buildFromSource()
	}
}

// ── platform installers ──────────────────────────────────

func installOnDarwin() error {
	if _, err := exec.LookPath("brew"); err == nil {
		return runInstall("brew", "install", "llama.cpp")
	}
	// Homebrew on Linux or missing; fall back to source build.
	return buildFromSource()
}

func installOnLinux() error {
	pkgManagers := []struct {
		cmd  string
		args []string
		desc string
	}{
		{"apt-get", []string{"install", "-y", "llama.cpp"}, "apt"},
		{"dnf", []string{"install", "-y", "llama.cpp"}, "dnf"},
		{"pacman", []string{"-S", "--noconfirm", "llama.cpp"}, "pacman"},
		{"zypper", []string{"install", "-y", "llama.cpp"}, "zypper"},
	}

	for _, pm := range pkgManagers {
		if _, err := exec.LookPath(pm.cmd); err != nil {
			continue
		}
		fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Installing llama.cpp via %s...\n", pm.desc)
		args := append([]string{pm.cmd}, pm.args...)
		cmd := exec.Command("sudo", args...)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err == nil {
			return nil
		}
		fmt.Fprintf(os.Stderr, "\033[33m==>\033[0m %s install failed, trying source build...\n", pm.desc)
		break
	}
	return buildFromSource()
}

func installOnWindows() error {
	if _, err := exec.LookPath("vcpkg"); err == nil {
		return runInstall("vcpkg", "install", "llama.cpp")
	}
	return buildFromSource()
}

// ── source build (universal fallback) ────────────────────

func buildFromSource() error {
	if _, err := exec.LookPath("cmake"); err != nil {
		return fmt.Errorf("cmake not found; install cmake first")
	}
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found; install git first")
	}
	if findCxxCompiler() == "" {
		return fmt.Errorf("C++ compiler not found (install g++, clang++, or MSVC)")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	srcDir := filepath.Join(home, ".local", "src", "llama.cpp")
	binDir := filepath.Join(home, ".local", "bin")
	installPrefix := filepath.Join(home, ".local")

	if err := os.RemoveAll(srcDir); err != nil {
		return fmt.Errorf("cannot clean source directory: %w", err)
	}
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return fmt.Errorf("cannot create source directory: %w", err)
	}

	fmt.Fprint(os.Stderr, "\033[32m==>\033[0m Cloning llama.cpp...\n")
	clone := exec.Command("git", "clone", "--depth=1",
		"https://github.com/ggerganov/llama.cpp", srcDir)
	clone.Stdout = os.Stderr
	clone.Stderr = os.Stderr
	if err := clone.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	fmt.Fprint(os.Stderr, "\033[32m==>\033[0m Configuring cmake build...\n")
	cfg := exec.Command("cmake", "-B", filepath.Join(srcDir, "build"),
		"-DCMAKE_BUILD_TYPE=Release",
		"-DCMAKE_INSTALL_PREFIX="+installPrefix,
		"-DLLAMA_BUILD_TESTS=OFF",
		"-DLLAMA_BUILD_EXAMPLES=ON",
		"-DLLAMA_BUILD_SERVER=OFF",
		srcDir)
	cfg.Stdout = os.Stderr
	cfg.Stderr = os.Stderr
	if err := cfg.Run(); err != nil {
		return fmt.Errorf("cmake configure failed: %w", err)
	}

	fmt.Fprint(os.Stderr, "\033[32m==>\033[0m Building llama.cpp (this may take a while)...\n")
	bld := exec.Command("cmake", "--build", filepath.Join(srcDir, "build"),
		"--config", "Release", "--target", "llama-cli", "--parallel")
	bld.Stdout = os.Stderr
	bld.Stderr = os.Stderr
	if err := bld.Run(); err != nil {
		return fmt.Errorf("cmake build failed: %w", err)
	}

	// Install to ~/.local so llama-cli lands in ~/.local/bin
	inst := exec.Command("cmake", "--install", filepath.Join(srcDir, "build"),
		"--config", "Release")
	inst.Stdout = os.Stderr
	inst.Stderr = os.Stderr
	if err := inst.Run(); err != nil {
		return fmt.Errorf("cmake install failed (binary built but not copied): %w", err)
	}

	// Ensure dir is created even if cmake --install didn't do it
	_ = os.MkdirAll(binDir, 0755)

	fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m llama-cli installed to %s\n", filepath.Join(binDir, "llama-cli"))
	fmt.Fprintf(os.Stderr, "\033[33m==>\033[0m Make sure %s is in your PATH\n", binDir)
	return nil
}

// ── small helpers ────────────────────────────────────────

func runInstall(name string, args ...string) error {
	fmt.Fprintf(os.Stderr, "\033[32m==>\033[0m Installing llama.cpp via %s...\n", name)
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s install failed: %w", name, err)
	}
	return nil
}

func findCxxCompiler() string {
	for _, name := range []string{"g++", "clang++", "c++"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("cl"); err == nil {
			return "cl"
		}
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
