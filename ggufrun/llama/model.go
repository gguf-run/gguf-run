package llama

import (
	"fmt"
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

	// Allocate a default-initialized model params buffer on the C heap
	// TODO: use ffi for proper struct-by-value passing
	pathBytes := append([]byte(path), 0)

	handle := LlamaModelLoadFromFile(&pathBytes[0], nil)
	if handle == 0 {
		return nil, fmt.Errorf("failed to load model from %s", path)
	}

	return &Model{handle: handle}, nil
}

// Free releases the model resources.
func (m *Model) Free() {
	if m.handle != 0 {
		LlamaModelFree(m.handle)
		m.handle = 0
	}
}

// NewContext creates a new inference context for this model.
func (m *Model) NewContext(ctxSize int32) (*Context, error) {
	// TODO: use ffi for proper struct-by-value passing
	// Passing nil for params means we get default context params
	handle := LlamaInitFromModel(m.handle, nil)
	if handle == 0 {
		return nil, fmt.Errorf("failed to create context")
	}

	return &Context{handle: handle, model: m}, nil
}

// Info returns model metadata.
func (m *Model) Info() ModelInfo {
	var buf [1024]byte
	LlamaModelDesc(m.handle, &buf[0], 1024)

	return ModelInfo{
		Name:      string(buf[:]),
		NVocab:    LlamaModelNVocab(m.handle),
		NEmbd:     LlamaModelNEmbd(m.handle),
		NLayers:   LlamaModelNLayer(m.handle),
		SizeBytes: int64(LlamaModelSize(m.handle)),
	}
}

// Tokenize converts text to tokens.
func (m *Model) Tokenize(text string, addBos, special bool) ([]int32, error) {
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
	if c.handle != 0 {
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
	// TODO: implement using ffi for struct-by-value support
	// This requires:
	//   1. llama_batch_get_one to create a batch
	//   2. llama_decode to process it
	//   3. llama_get_logits_ith to get logits
	//   4. sampling functions
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
