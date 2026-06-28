package ggufrun

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Checksum struct {
	SHA256 string `json:"sha256"`
}

type Runtime struct {
	MCP_Servers  []string `json:"mcp_servers,omitempty"`
	MemoryMB     int      `json:"memory_mb,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

type GgufRef struct {
	Source    string `json:"source"`
	ModelID   string `json:"model_id,omitempty"`
	URL       string `json:"url"`
	Filename  string `json:"filename"`
	Quant     string `json:"quant,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
}

type CgpManifest struct {
	Name              string     `json:"name"`
	Version           string     `json:"version"`
	Description       string     `json:"description,omitempty"`
	Author            string     `json:"author,omitempty"`
	License           string     `json:"license,omitempty"`
	MinCognitiveOSVer string     `json:"min_cognitiveos_version,omitempty"`
	Dependencies      []string   `json:"dependencies,omitempty"`
	Runtime           *Runtime   `json:"runtime,omitempty"`
	Gguf              *GgufRef   `json:"gguf,omitempty"`
	Prompts           []string   `json:"prompts,omitempty"`
	Tools             []string   `json:"tools,omitempty"`
	Checksum          *Checksum  `json:"checksum,omitempty"`
}

func ReadGgufRef(path string) (*GgufRef, error) {
	var data []byte

	if strings.HasSuffix(path, ".cgp") {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		gr, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("open .cgp: %w", err)
		}
		defer gr.Close()
		tr := tar.NewReader(gr)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("read .cgp: %w", err)
			}
			if hdr.Name == "cognitive.json" {
				data, err = io.ReadAll(tr)
				if err != nil {
					return nil, err
				}
				break
			}
		}
	} else {
		p := filepath.Join(path, "cognitive.json")
		var err error
		data, err = os.ReadFile(p)
		if err != nil {
			return nil, err
		}
	}

	if data == nil {
		return nil, fmt.Errorf("cognitive.json not found in %s", path)
	}

	var manifest CgpManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("invalid cognitive.json: %w", err)
	}
	if manifest.Gguf == nil || manifest.Gguf.URL == "" {
		return nil, fmt.Errorf("no gguf URL reference in %s", path)
	}
	return manifest.Gguf, nil
}

func BuildCgp(name string, ref *GgufRef, outputDir string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "gguf-cgp-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	for _, d := range []string{"prompts", "tools", "weights"} {
		if err := os.MkdirAll(filepath.Join(tmpDir, d), 0755); err != nil {
			return "", err
		}
	}

	memoryMB := int(ref.SizeBytes / (1 << 20) * 2)
	if memoryMB < 128 {
		memoryMB = 128
	}

	modelID := ref.ModelID
	if modelID == "" {
		modelID = name
	}

	manifest := CgpManifest{
		Name:        name,
		Version:     "1.0.0",
		Description: fmt.Sprintf("%s - GGUF model", modelID),
		Author:      "gguf-run",
		License:     "MIT",
		Runtime: &Runtime{
			MemoryMB:     memoryMB,
			Capabilities: []string{"text-generation"},
		},
		Gguf:    ref,
		Prompts: []string{"prompts/system.md"},
		Checksum: &Checksum{
			SHA256: "0000000000000000000000000000000000000000000000000000000000000000",
		},
	}

	if err := writeManifest(tmpDir, &manifest); err != nil {
		return "", err
	}

	systemPrompt := fmt.Sprintf("You are %s, a helpful AI assistant powered by a GGUF model.", modelID)
	if err := os.WriteFile(filepath.Join(tmpDir, "prompts", "system.md"), []byte(systemPrompt), 0644); err != nil {
		return "", err
	}
	for _, f := range []string{"tools/.gitkeep", "weights/.gitkeep"} {
		if err := os.WriteFile(filepath.Join(tmpDir, f), []byte{}, 0644); err != nil {
			return "", err
		}
	}

	// First pass: build to buffer to compute checksum
	var buf []byte
	{
		var b strings.Builder
		if err := tarGzDir(&b, tmpDir); err != nil {
			return "", fmt.Errorf("build archive: %w", err)
		}
		buf = []byte(b.String())
	}

	h := sha256.Sum256(buf)
	manifest.Checksum.SHA256 = fmt.Sprintf("%x", h[:])

	if err := writeManifest(tmpDir, &manifest); err != nil {
		return "", err
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	outPath := filepath.Join(outputDir, name+".cgp")
	out, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("create output file: %w", err)
	}
	defer out.Close()

	if err := tarGzDir(out, tmpDir); err != nil {
		return "", fmt.Errorf("build final archive: %w", err)
	}

	return outPath, nil
}

func writeManifest(dir string, m *CgpManifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "cognitive.json"), data, 0644)
}

func tarGzDir(w io.Writer, dir string) error {
	gw := gzip.NewWriter(w)
	tw := tar.NewWriter(gw)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel
		if info.IsDir() {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if !info.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if _, err := tw.Write(data); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gw.Close()
}
