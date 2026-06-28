# .cgp Package Management Tutorial

[gguf-run](https://github.com/gguf-run/gguf-run) can package GGUF model references into
`.cgp` (Cognitive Patch) archives — the standard package format for the
[CognitiveOS](https://cognitive-os.org) ecosystem. This tutorial covers the
full lifecycle: creating, inspecting, installing, and running `.cgp` packages.

---

## Table of Contents

1. [What is a .cgp file?](#1-what-is-a-cgp-file)
2. [Quick start](#2-quick-start)
3. [Creating a .cgp package](#3-creating-a-cgp-package)
4. [Inspecting a .cgp package](#4-inspecting-a-cgp-package)
5. [Installing a .cgp package with cpm](#5-installing-a-cgp-package-with-cpm)
6. [Running models from .cgp packages](#6-running-models-from-cgp-packages)
7. [Publishing to a registry](#7-publishing-to-a-registry)
8. [Reference: cognitive.json fields](#8-reference-cognitivejson-fields)
9. [Testing in Alpine Docker](#9-testing-in-alpine-docker)

---

## 1. What is a .cgp file?

A `.cgp` file is a **gzipped tar archive** (tar.gz) that contains:

```
my-model.cgp
├── cognitive.json          # Package manifest (REQUIRED)
├── prompts/system.md       # Default system prompt (OPTIONAL)
├── tools/                  # MCP server binaries (OPTIONAL)
└── weights/                # Model weights (OPTIONAL)
```

The `cognitive.json` manifest tells the system what the package provides:
what model it references, how much memory it needs, what hardware it requires,
and optional MCP servers or prompt templates.

For GGUF models, the `.cgp` contains **metadata only** — the actual GGUF
binary stays in `gguf-run`'s cache. The `remote` key under `weights` inside
`cognitive.json` points to the Hugging Face download URL:

```json
{
  "name": "tinyllama-1.1b",
  "version": "1.0.0",
  "description": "TinyLlama 1.1B Chat Q4_K_M GGUF",
  "weights": {
    "remote": {
      "source": "huggingface",
      "url": "https://huggingface.co/TheBloke/.../tinyllama-1.1b.Q4_K_M.gguf",
      "filename": "tinyllama-1.1b.Q4_K_M.gguf",
      "quant": "Q4_K_M",
      "size_bytes": 637800000
    }
  },
  "checksum": {
    "sha256": "fab3c05314bc94605c9c041c6a2f7dbc..."
  }
}
```

This means `.cgp` files are **tiny** (a few KB) — they can be emailed,
checked into git, published to registries, or shared on forums without
moving gigabytes of model data.

---

## 2. Quick start

```bash
# 1. Package a model by name (searches Hugging Face)
gguf-run package tinyllama

# 2. Install the package with cpm
cpm install tinyllama-1.1b.cgp

# 3. Run the model from the installed package
gguf-run run /cognitiveos/patches/tinyllama-1.1b/

# Or run the .cgp file directly
gguf-run run ./tinyllama-1.1b.cgp -p "Hello"
```

---

## 3. Creating a .cgp package

### 3a. From a search query

The simplest way — `gguf-run` searches Hugging Face and picks the best
Q4_K_M quantized file automatically:

```bash
gguf-run package tinyllama
gguf-run package qwen2.5-1.5b
gguf-run package phi-2
```

Output:
```
==> Searching: tinyllama
==> Selected: tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf
==> Package created: tinyllama-1.1b-chat-v1.0.Q4_K_M.cgp
==> Install with: cpm install tinyllama-1.1b-chat-v1.0.Q4_K_M.cgp
```

### 3b. From a direct URL

```bash
gguf-run package https://huggingface.co/TheBloke/Phi-3-mini-4k-instruct-GGUF/resolve/main/Phi-3-mini-4k-instruct.Q4_K_M.gguf
```

`gguf-run` will issue a HEAD request to discover the file size and
package it into a `.cgp`.

### 3c. From a local .gguf file

```bash
gguf-run package ~/.cache/gguf/my-model.q4_k_m.gguf
```

This computes the SHA-256 hash and file size locally. The resulting `.cgp`
will have an empty `url` field — it's suitable for local-only use or for
publishing alongside the actual GGUF file on a registry.

### 3d. Custom package name and output directory

```bash
gguf-run package tinyllama --name my-tinyllama --output ./packages/
```

| Flag | Default | Description |
|------|---------|-------------|
| `--name` | (from filename) | Override the package name in cognitive.json |
| `--output` | `.` | Output directory for the `.cgp` file |

### 3e. Package format detail

The created file (`<name>.cgp`) is a standard tar.gz archive:

```bash
tar -tzf tinyllama-1.1b.cgp
```

```
cognitive.json
prompts/
prompts/system.md
tools/
tools/.gitkeep
weights/
weights/.gitkeep
```

---

## 4. Inspecting a .cgp package

### Extract and view the manifest

```bash
tar -xzf tinyllama-1.1b.cgp -O cognitive.json | jq .
```

```json
{
  "name": "tinyllama-1.1b-chat-v1.0.Q4_K_M",
  "version": "1.0.0",
  "description": "TinyLlama-1.1B-Chat-v1.0-GGUF - GGUF model",
  "author": "gguf-run",
  "license": "MIT",
  "runtime": {
    "memory_mb": 1276,
    "capabilities": ["text-generation"]
  },
  "weights": {
    "remote": {
      "source": "huggingface",
      "model_id": "TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF",
      "url": "https://huggingface.co/TheBloke/.../tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf",
      "filename": "tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf",
      "quant": "Q4_K_M",
      "size_bytes": 637800000
    }
  },
  "prompts": ["prompts/system.md"],
  "checksum": {
    "sha256": "fab3c05314bc94605c9c041c6a2f7dbc..."
  }
}
```

### View the default system prompt

```bash
tar -xzf tinyllama-1.1b.cgp -O prompts/system.md
```

```
You are TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF, a helpful AI assistant
powered by a GGUF model.
```

---

## 5. Installing a .cgp package with cpm

[cpm](https://github.com/CognitiveOS-Project/cpm) is the Cognitive Package
Manager. It installs, removes, and manages `.cgp` packages on your system.

### Install from a local file

```bash
cpm install ./tinyllama-1.1b.cgp
```

This:
1. Validates the `cognitive.json` manifest against the schema
2. Checks that the archive SHA-256 matches the manifest
3. Audits hardware requirements (RAM, storage, NPU)
4. Extracts to `/cognitiveos/patches/tinyllama-1.1b/`
5. Registers any MCP servers (if the package has them)

### Install from a registry

```bash
cpm install tinyllama-1.1b          # resolves from configured registry
cpm install tinyllama-1.1b --registry official.eu   # from EU mirror
```

### List installed packages

```bash
cpm list
```

```
tinyllama-1.1b  1.0.0  GGUF model (text-generation)
```

### Remove a package

```bash
cpm remove tinyllama-1.1b
```

### View package details

```bash
cpm info tinyllama-1.1b
```

---

## 6. Running models from .cgp packages

### Run a .cgp file directly

```bash
gguf-run run ./tinyllama-1.1b.cgp
gguf-run run ./tinyllama-1.1b.cgp -p "What is the capital of France?"
```

`gguf-run` detects the `.cgp` extension, reads the `gguf.url` from inside
the archive, downloads the model if not cached, and launches llama-cli.

### Run an extracted cgp directory

After `cpm install`:

```bash
gguf-run run /cognitiveos/patches/tinyllama-1.1b/
```

`gguf-run` detects the `cognitive.json` in the directory, reads the
`gguf.url`, and runs the model — all without needing to know cpm's
internal paths.

### Run with the server binary

```bash
gguf-run server ./tinyllama-1.1b.cgp --addr :8080
```

This uses `llama-server` instead of `llama-cli`, exposing the model
via an HTTP API compatible with the OpenAI chat completions format.

### Full example: package → install → run

```bash
# Step 1 — Package the model
gguf-run package qwen2.5-1.5b --output ./packages/

# Step 2 — Install via cpm
cpm install ./packages/qwen2.5-1.5b.cgp

# Step 3 — Run interactively
gguf-run run /cognitiveos/patches/qwen2.5-1.5b/

# Step 4 — Single-shot prompt
gguf-run run /cognitiveos/patches/qwen2.5-1.5b/ -p "Write a haiku about Go"
```

### What happens under the hood

When you run `gguf-run run model.cgp`:

```
1. Detect .cgp extension or cognitive.json in the directory
2. Open archive (or read directory)
3. Parse cognitive.json
4. Extract weights.remote.url field
5. Download GGUF to cache (~/.cache/gguf/) if not cached
6. Validate GGUF magic bytes
7. Find llama-cli (or prompt to install via cpm/pkg manager)
8. Launch llama-cli with -m <model> -p <prompt>
```

---

## 7. Publishing to a registry

Once you have a `.cgp` file, you can publish it to a CognitiveOS
package registry for others to discover and install.

```bash
cpm publish ./tinyllama-1.1b.cgp --registry https://my-registry.example.com/v1
```

This requires:
- A registry server (see [registry-server](https://github.com/CognitiveOS-Project/registry-server))
- Authentication (token from `~/.cognitiveos/cpm-credentials.json` or `CPM_TOKEN` env var)

Others can then install it:

```bash
cpm install tinyllama-1.1b
cpm install tinyllama-1.1b --registry https://my-registry.example.com
```

---

## 8. Reference: cognitive.json fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Package name |
| `version` | string | yes | SemVer version |
| `description` | string | no | Human-readable description |
| `author` | string | no | Package author |
| `license` | string | no | SPDX license identifier |
| `weights.remote` | object | no* | Remote GGUF model reference |
| `weights.remote.source` | string | no | Source platform (`huggingface`, `local`) |
| `weights.remote.model_id` | string | no | Hugging Face model ID |
| `weights.remote.url` | string | no* | Download URL for the GGUF file |
| `weights.remote.filename` | string | no | GGUF filename |
| `weights.remote.quant` | string | no | Quantization (e.g. `Q4_K_M`) |
| `weights.remote.size_bytes` | int | no | File size in bytes |
| `weights.remote.sha256` | string | no | SHA-256 checksum of the GGUF file |
| `runtime.memory_mb` | int | no | Suggested RAM allocation |
| `runtime.capabilities` | [string] | no | Required capabilities |
| `checksum.sha256` | string | yes | SHA-256 of the `.cgp` archive itself |
| `prompts` | [string] | no | Paths to prompt files |
| `tools` | [string] | no | Glob patterns for tool executables |

\* At least one of `weights.remote.url` or actual files in `weights/`
   is expected for packages that provide model inference.

---

## 9. Testing in Alpine Docker

Use an Alpine container to verify the `apk` install path and source-build
fallback in a clean Linux environment.

### Quick smoke test (interactive)

```bash
docker run -it --rm alpine:edge sh

# inside the container:
apk add go git cmake g++ linux-headers
go install github.com/gguf-run/gguf-run@latest
export PATH="/root/go/bin:$PATH"

# test the full pipeline: search → download → run
gguf-run run tinyllama -p "hello"
```

This exercises:

| Code path | Alpine behavior |
|-----------|----------------|
| `privilegeCmd()` | Returns `""` — running as root in Docker, no `sudo` |
| `installOnLinux()` → `apk` | `apk add llama.cpp` from `edge/community` |
| `findBinary()` | `exec.LookPath` + `~/.local/bin` fallback checks |
| `SearchAndDownload()` | Hugging Face API search + download |
| `Run()` | Spawns `llama-cli` with the downloaded model |

### With a Dockerfile

```dockerfile
FROM alpine:edge

RUN apk add --no-cache go git cmake g++ linux-headers \
    && go install github.com/gguf-run/gguf-run@latest \
    && rm -rf /var/cache/apk/*

ENV PATH="/root/go/bin:${PATH}"

ENTRYPOINT ["gguf-run"]
```

Build and run:

```bash
docker build -t gguf-run-test .
docker run --rm gguf-run-test run tinyllama -p "hello"
```

### Test the source-build fallback

If you want to verify the `cmake` source build path (skipping the `apk`
package entirely), use an older Alpine release where `llama.cpp` is not
in the repos:

```bash
docker run -it --rm alpine:3.20 sh

apk add go git cmake g++ linux-headers
go install github.com/gguf-run/gguf-run@latest

# llama.cpp won't be in the repo, so InstallLlamaCpp() will
# fall through to buildFromSource() which clones, cmakes, and
# installs to ~/.local/bin
gguf-run run tinyllama -p "hello"
```

---

## Related resources

- [gguf-run](https://github.com/gguf-run/gguf-run) — the tool that
  creates `.cgp` packages
- [cpm](https://github.com/CognitiveOS-Project/cpm) — Cognitive Package
  Manager (installs `.cgp` files)
- [cgp-format spec](https://github.com/CognitiveOS-Project/product-specs/blob/main/specs/cgp-format.md)
  — the full `.cgp` format specification
- [cgp-template](https://github.com/CognitiveOS-Project/cgp-template)
  — developer template for creating `.cgp` packages manually
- [cpm spec](https://github.com/CognitiveOS-Project/product-specs/blob/main/specs/cpm-spec.md)
  — full cpm lifecycle specification
- [registry API](https://github.com/CognitiveOS-Project/product-specs/blob/main/specs/registry-api.md)
  — publishing and resolving packages from registries
- [CognitiveOS](https://cognitive-os.org) — the CognitiveOS project
