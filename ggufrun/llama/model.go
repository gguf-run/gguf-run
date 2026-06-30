package llama

import (
	"fmt"
	"strings"
)

// Model represents a loaded GGUF model.
type Model struct {
	handle uintptr
}

// LoadModel loads a GGUF model from the given path.
func LoadModel(path string) (*Model, error) {
	if !IsLoaded() {
		return nil, ErrLibraryNotLoaded
	}
	if LlamaModelLoadFromFile == nil {
		return nil, fmt.Errorf("llama_model_load_from_file not available in this library version")
	}

	pathBytes := append([]byte(path), 0)

	handle := LlamaModelLoadFromFile(&pathBytes[0], nil)
	if handle == 0 {
		return nil, fmt.Errorf("failed to load model from %s", path)
	}

	return &Model{handle: handle}, nil
}

// Free releases the model resources.
func (m *Model) Free() {
	if m.handle != 0 && LlamaModelFree != nil {
		LlamaModelFree(m.handle)
		m.handle = 0
	}
}

// NewContext creates a new inference context for this model.
func (m *Model) NewContext(ctxSize int32) (*Context, error) {
	if LlamaInitFromModel == nil {
		return nil, fmt.Errorf("llama_init_from_model not available in this library version")
	}

	handle := LlamaInitFromModel(m.handle, nil)
	if handle == 0 {
		return nil, fmt.Errorf("failed to create context")
	}

	return &Context{handle: handle, model: m}, nil
}

// Info returns model metadata.
func (m *Model) Info() ModelInfo {
	info := ModelInfo{Name: "model"}

	var buf [1024]byte
	if LlamaModelDesc != nil {
		LlamaModelDesc(m.handle, &buf[0], 1024)
		info.Description = string(buf[:])
		// Use first 100 chars as name
		if name := strings.SplitN(info.Description, " ", 2)[0]; name != "" {
			info.Name = name
		}
	}
	if LlamaModelNVocab != nil {
		info.NVocab = LlamaModelNVocab(m.handle)
	}
	if LlamaModelNEmbd != nil {
		info.NEmbd = LlamaModelNEmbd(m.handle)
	}
	if LlamaModelNLayer != nil {
		info.NLayers = LlamaModelNLayer(m.handle)
	}
	if LlamaModelSize != nil {
		info.SizeBytes = int64(LlamaModelSize(m.handle))
	}

	return info
}

// Tokenize converts text to tokens.
func (m *Model) Tokenize(text string, addBos, special bool) ([]int32, error) {
	if LlamaTokenize == nil {
		return nil, fmt.Errorf("llama_tokenize not available in this library version")
	}

	maxTokens := int32(len(text)/2 + 10)
	tokens := make([]int32, maxTokens)

	textBytes := append([]byte(text), 0)
	n := LlamaTokenize(m.handle, &textBytes[0], int32(len(textBytes)-1), &tokens[0], maxTokens, addBos, special)
	if n < 0 {
		return nil, fmt.Errorf("tokenize failed")
	}

	return tokens[:n], nil
}

// TokenToPiece converts a token to its string representation.
func (m *Model) TokenToPiece(token int32, lstrip, special bool) string {
	if LlamaTokenToPiece == nil {
		return ""
	}

	var buf [256]byte
	lstripInt := int32(0)
	if lstrip {
		lstripInt = 1
	}
	n := LlamaTokenToPiece(m.handle, token, &buf[0], 256, lstripInt, special)
	if n <= 0 {
		return ""
	}
	return string(buf[:n])
}

// Special token getters.
func (m *Model) TokenBOS() int32 {
	if LlamaTokenBos != nil {
		return LlamaTokenBos(m.handle)
	}
	return 1
}

func (m *Model) TokenEOS() int32 {
	if LlamaTokenEos != nil {
		return LlamaTokenEos(m.handle)
	}
	return 2
}

func (m *Model) TokenEOT() int32 {
	if LlamaTokenEot != nil {
		return LlamaTokenEot(m.handle)
	}
	return -1
}

func (m *Model) TokenNL() int32 {
	if LlamaTokenNl != nil {
		return LlamaTokenNl(m.handle)
	}
	return 13
}

func (m *Model) TokenSEP() int32 {
	if LlamaTokenSep != nil {
		return LlamaTokenSep(m.handle)
	}
	return -1
}

func (m *Model) TokenPAD() int32 {
	if LlamaTokenPad != nil {
		return LlamaTokenPad(m.handle)
	}
	return -1
}

func (m *Model) TokenMASK() int32 {
	if LlamaTokenMask != nil {
		return LlamaTokenMask(m.handle)
	}
	return -1
}

// Context represents an inference context.
type Context struct {
	handle uintptr
	model  *Model
}

// Free releases the context resources.
func (c *Context) Free() {
	if c.handle != 0 && LlamaFree != nil {
		LlamaFree(c.handle)
		c.handle = 0
	}
}

// Model returns the model associated with this context.
func (c *Context) Model() *Model {
	return c.model
}

// Generate runs generation - NOT YET IMPLEMENTED (needs ffi for struct passing).
func (c *Context) Generate(opts GenerateOptions) (string, error) {
	return "", fmt.Errorf("inference via purego not yet implemented - needs ffi struct support")
}

// tokenize delegates to the model's tokenizer.
func (c *Context) tokenize(text string) ([]int32, error) {
	return c.model.Tokenize(text, true, false)
}

// tokenToPiece delegates to the model's detokenizer.
func (c *Context) tokenToPiece(token int32) string {
	return c.model.TokenToPiece(token, false, false)
}
