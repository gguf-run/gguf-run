# gguf-run

Search, download, and run GGUF models with [llama.cpp](https://github.com/ggerganov/llama.cpp) in one command.

```
gguf-run -q tinyllama        # interactive chat
gguf-run -q qwen2.5-1.5b -p "Hi"  # single prompt
```

## Why this tool?

Finding and running GGUF models typically involves multiple steps:
search Hugging Face, pick a quantization, download, then invoke
llama.cpp. `gguf-run` automates all of that:

```bash
gguf-run -q tinyllama
```

This will:
1. Search Hugging Face for TinyLlama GGUF files
2. Auto-select the best Q4_K_M quantized version
3. Download to `~/.cache/gguf/` (if not already cached)
4. Validate the file (checks GGUF magic bytes)
5. Launch llama-cli in interactive chat mode

## Installation

Requires [Go 1.21+](https://go.dev/dl/).

```bash
go install github.com/gguf-run/gguf-run@latest
```

Make sure `$GOPATH/bin` is in your PATH, then:

```bash
gguf-run -q tinyllama
```

### Dependencies

- **llama.cpp** — installed automatically via Homebrew on first run if not found
- **[finder](https://github.com/gguf-run/finder)** — installed separately for model search, or use `-m` for direct paths

## Usage

```
gguf-run -q <query> [-p <prompt>] [-m <path>] [--cache-dir <dir>] [-- <llama-cli args>]
```

| Flag | Default | Description |
|------|---------|-------------|
| `-q` | (see note) | Hugging Face search query (requires [finder](https://github.com/gguf-run/finder)) |
| `-m` | (see note) | Direct model path or download URL |
| `-p` | (interactive) | Single-shot prompt; exits after generating response |
| `--cache-dir` | `~/.cache/gguf/` | Directory for downloaded models |

Either `-q` or `-m` must be provided. Extra arguments after `--` are
passed through to llama-cli.

### Examples

**Interactive chat (auto-search):**
```bash
gguf-run -q tinyllama
```

**Single-shot prompt:**
```bash
gguf-run -q qwen2.5-1.5b -p "What is the capital of France?"
```

**Use a local GGUF file:**
```bash
gguf-run -m ~/models/my-model.q4_k_m.gguf
```

**Use a direct download URL:**
```bash
gguf-run -m https://huggingface.co/org/model/resolve/main/file.gguf
```

**Pass extra llama.cpp flags:**
```bash
gguf-run -q phi -- --temp 0.8 --ctx-size 4096 -ngl 999
```

## How it works

1. Finds or installs `llama-cli` (via PATH, `LLAMACPP_DIR`, or Homebrew)
2. Resolves the model:
   - With `-m`: uses the given path or downloads the URL
   - With `-q`: calls [finder](https://github.com/gguf-run/finder) to search Hugging Face, picks Q4_K_M
3. Downloads to cache directory if not present, validates GGUF magic bytes
4. Launches `llama-cli` in interactive mode or with `--single-turn`

## Environment

| Variable | Description |
|----------|-------------|
| `LLAMACPP_DIR` | Path to llama.cpp installation (looks for `bin/llama-cli`) |
| `XDG_CACHE_HOME` | Override cache base directory (default: `~/.cache`) |

## Related

- [finder](https://github.com/gguf-run/finder) — search Hugging Face Hub for GGUF files (standalone CLI)

## License

MIT
