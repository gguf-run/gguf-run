# gguf-run

Search, download, and run GGUF models with [llama.cpp](https://github.com/ggerganov/llama.cpp) in one command. Create `.cgp` packages for the [CognitiveOS](https://cognitive-os.org) ecosystem.

## Installation

```bash
go install github.com/gguf-run/gguf-run@latest
```

Requires [Go 1.22+](https://go.dev/dl/). Make sure `$GOPATH/bin` is in your `PATH`.

## Quick start

```bash
go install github.com/gguf-run/gguf-run@latest
gguf-run run tinyllama                     # installs llama.cpp, then runs
gguf-run run qwen2.5-1.5b -p "Hi"          # second run: fast, no install
gguf-run server phi-2 --addr :8080         # HTTP API server
gguf-run list                              # show cached models
```

llama.cpp is installed **automatically on first launch** — no separate step needed.

## Subcommands

### `run <model>` — search, download, and run

```bash
gguf-run run <model> [-p <prompt>] [--cache-dir <dir>] [-- <llama-cli args>]
```

| Flag | Default | Description |
|------|---------|-------------|
| `-p` | (interactive) | Single-shot prompt; exits after generating response |
| `--cache-dir` | `~/.cache/gguf/` | Model cache directory |
| `--` | — | Extra arguments passed through to `llama-cli` |

```bash
gguf-run run tinyllama                              # interactive chat
gguf-run run qwen2.5-1.5b -p "What is the capital?"  # single prompt
gguf-run run model.cgp                              # from .cgp package
gguf-run run /cognitiveos/patches/tinyllama/        # from cpm-installed dir
gguf-run run phi -- --temp 0.8 --ctx-size 4096      # extra llama-cli flags
```

### `server <model>` — serve via HTTP API

```bash
gguf-run server <model> [--addr <host:port>] [--cache-dir <dir>] [-- <llama-server args>]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--addr` | `:8080` | Server listen address |
| `--cache-dir` | `~/.cache/gguf/` | Model cache directory |
| `--` | — | Extra arguments passed through to `llama-server` |

```bash
gguf-run server phi-2 --addr :8080
gguf-run server tinyllama.cgp --addr 0.0.0.0:8080 -- --ngl 999
```

### `list` — list cached models

```bash
gguf-run list [--cache-dir <dir>]
```

Shows `NAME`, `SIZE`, and `MODIFIED` for all `.gguf` files in the cache directory.

### `pull <model>` — download without running

```bash
gguf-run pull <model> [--cache-dir <dir>]
```

Downloads the model to the cache (or resolves from a `.cgp` package) without launching llama-cli.

### `rm <model>` — remove a cached model

```bash
gguf-run rm <model> [--cache-dir <dir>]
```

Removes the model file from cache. The `<model>` argument matches by:
- Exact filename (e.g., `model.q4_k_m.gguf`)
- Name without `.gguf` extension (e.g., `model.q4_k_m`)
- Partial name (e.g., `tinyllama` matches any cached file containing "tinyllama")

### `install` — re-install / upgrade llama.cpp

Normally not needed — llama.cpp is installed automatically on first run.
Use this to upgrade or re-install:

```bash
gguf-run install
```

| Platform | Method | Fallback |
|----------|--------|----------|
| macOS | `brew install llama.cpp` | Build from source via cmake |
| Linux (Debian/Ubuntu) | `sudo apt-get install llama.cpp` | Build from source via cmake |
| Linux (Fedora) | `sudo dnf install llama.cpp` | Build from source via cmake |
| Linux (Arch) | `sudo pacman -S llama.cpp` | Build from source via cmake |
| Linux (Alpine) | `apk add llama.cpp` | Build from source via cmake |
| Linux (openSUSE) | `sudo zypper install llama.cpp` | Build from source via cmake |
| Windows | `vcpkg install llama.cpp` | Build from source via cmake |

### `package <model>` — create a `.cgp` package

```bash
gguf-run package <model> [--output <dir>] [--name <name>]
```

Creates a `.cgp` (Cognitive Patch) archive with model metadata and a download URL reference. The actual GGUF file is **not** included — the `.cgp` is a lightweight wrapper for distribution via [cpm](https://github.com/CognitiveOS-Project/cpm).

| Flag | Default | Description |
|------|---------|-------------|
| `--output` | `.` | Output directory for the `.cgp` file |
| `--name` | (from filename) | Override the package name in `cognitive.json` |

The `<model>` argument can be:

| Input | Behavior |
|-------|----------|
| `package tinyllama` | Search Hugging Face → pick best Q4_K_M → create `.cgp` with download URL |
| `package https://...` | Extract filename and size from the URL → create `.cgp` |
| `package ./model.gguf` | Compute SHA-256 and file size → create `.cgp` (local reference) |

```bash
gguf-run package tinyllama                        # search + package
gguf-run package --output ./packages/ qwen2.5-1.5b
gguf-run package --name my-model ./local.gguf      # local file with custom name
gguf-run package https://huggingface.co/.../model.gguf
```

Output is a standard tar.gz archive installable via `cpm install`:

```bash
cpm install ./tinyllama-1.1b.cgp
```

See [TUTORIAL.md](./TUTORIAL.md) for the full cgp management guide.

## Model resolution

When you pass `<model>` to `run`, `server`, or `pull`, it resolves in this order:

1. **URL** (`https://...`) → download directly
2. **.cgp file** (`model.cgp`) → read `cognitive.json` → extract `weights.remote.url` → download
3. **CGP directory** (directory with `cognitive.json`) → same as `.cgp` file
4. **Local .gguf file** → use directly
5. **Search query** → search Hugging Face Hub → pick Q4_K_M → download

## Legacy mode (backward compatible)

The original flag-based interface still works:

```bash
gguf-run -q <query> [-p <prompt>] [-m <path>] [--cache-dir <dir>] [-- <llama-cli args>]
```

| Flag | Description |
|------|-------------|
| `-q` | Hugging Face search query |
| `-m` | Direct model path or download URL |
| `-p` | Single-shot prompt (default: interactive) |
| `--cache-dir` | Model cache directory (default: `~/.cache/gguf/`) |

Either `-q` or `-m` must be provided.

```bash
gguf-run -q tinyllama
gguf-run -q qwen2.5-1.5b -p "What is the capital?"
gguf-run -m ~/models/model.q4_k_m.gguf
gguf-run -m https://huggingface.co/org/model/resolve/main/file.gguf
gguf-run -q phi -- --temp 0.8 --ctx-size 4096 -ngl 999
```

## Environment variables

| Variable | Description |
|----------|-------------|
| `LLAMACPP_DIR` | Path to llama.cpp installation (looks for `bin/llama-cli`) |
| `XDG_CACHE_HOME` | Override cache base directory (default: `~/.cache`) |

## Library

Use the `ggufrun` package in your Go application:

```go
import "github.com/gguf-run/gguf-run/ggufrun"

ctx := context.Background()

// Search for a model
results, err := ggufrun.Search(ctx, "tinyllama", 10)

// Download a model
path, err := ggufrun.Download(ctx, "https://...", "~/.cache/gguf/")

// Run with llama-cli
ggufrun.Run("/usr/local/bin/llama-cli", path, "-p", "Hello")

// Find or install llama.cpp
path := ggufrun.FindLlamaCli()
if path == "" {
    ggufrun.InstallLlamaCpp()
}

// List cached models
models, _ := ggufrun.ListCached("~/.cache/gguf/")
for _, m := range models {
    fmt.Println(m.Name, m.Size)
}

// Create .cgp packages
ref := &ggufrun.GgufRef{URL: "https://...", Filename: "model.gguf"}
path, _ := ggufrun.BuildCgp("my-model", ref, "./output/")

// Read .cgp references
ref, _ = ggufrun.ReadGgufRef("./model.cgp")
fmt.Println(ref.URL)
```

## Related

- [TUTORIAL.md](./TUTORIAL.md) — comprehensive guide to `.cgp` package management
- [finder](https://github.com/gguf-run/finder) — search Hugging Face Hub for GGUF files
- [cpm](https://github.com/CognitiveOS-Project/cpm) — Cognitive Package Manager (installs `.cgp` files)
- [cgp-format spec](https://github.com/CognitiveOS-Project/product-specs/blob/main/specs/cgp-format.md) — `.cgp` format specification
- [CognitiveOS](https://cognitive-os.org)

## License

MIT
