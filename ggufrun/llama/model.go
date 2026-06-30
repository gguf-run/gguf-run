package llama

import (
	"fmt"
	"strings"
	"unsafe"
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
	if LlamaModelLoadFromFileAddr == 0 {
		return nil, fmt.Errorf("llama_model_load_from_file not available in this library version")
	}

	pathBytes := append([]byte(path), 0)

	params := CModelParams{
		NGpuLayers: -1, // all layers
		MainGpu:   -1, // no preference
	}

	handle, err := LoadModelFFI(&pathBytes[0], &params)
	if err != nil {
		return nil, fmt.Errorf("failed to load model from %s: %w", path, err)
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
	if LlamaInitFromModelAddr == 0 {
		return nil, fmt.Errorf("llama_init_from_model not available in this library version")
	}

	params := CContextParams{
		NCtx:   uint32(ctxSize),
		NBatch: 512,
		NUBatch: 512,
	}

	handle, err := InitFromModelFFI(m.handle, &params)
	if err != nil {
		return nil, fmt.Errorf("failed to create context: %w", err)
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

// Generate runs text generation: tokenize prompt → prefill → autoregressive loop.
func (c *Context) Generate(opts GenerateOptions) (string, error) {
	if c.handle == 0 {
		return "", fmt.Errorf("context has been freed")
	}

	// Model metadata
	model := c.Model()
	nVocab := int(model.Info().NVocab)
	if nVocab <= 0 {
		nVocab = 32000
	}
	eosToken := model.TokenEOS()

	// 1. Tokenize prompt
	promptTokens, err := c.tokenize(opts.Prompt)
	if err != nil {
		return "", fmt.Errorf("tokenize: %w", err)
	}
	if len(promptTokens) == 0 {
		return "", fmt.Errorf("empty prompt")
	}

	// 2. Prefill: decode all prompt tokens at once
	{
		batch := buildBatch(promptTokens, true)
		if _, err := DecodeFFI(c.handle, &batch); err != nil {
			return "", fmt.Errorf("prefill: %w", err)
		}
	}

	// 3. Generation loop
	var output strings.Builder
	var lastTokens []int32
	pos := int32(len(promptTokens))
	maxTokens := opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2048
	}

	for i := int32(0); i < maxTokens; i++ {
		logitsPtr := c.getLogitsIth(0)
		if logitsPtr == nil {
			return output.String(), fmt.Errorf("nil logits at step %d", i)
		}
		logits := unsafe.Slice(logitsPtr, nVocab)

		nextToken := sampleToken(logits, opts.Temperature, opts.TopK,
			lastTokens, opts.RepeatPenalty)

		if nextToken == eosToken && !opts.IgnoreEOS {
			break
		}

		piece := model.TokenToPiece(nextToken, false, false)
		output.WriteString(piece)
		if opts.Callback != nil {
			opts.Callback(piece)
		}

		full := output.String()
		if checkStopSequences(full, opts.StopSequences) {
			break
		}

		lastTokens = append(lastTokens, nextToken)
		if len(lastTokens) > 64 {
			lastTokens = lastTokens[1:]
		}

		batch := buildSingleTokenBatch(nextToken, pos)
		pos++
		if _, err := DecodeFFI(c.handle, &batch); err != nil {
			return output.String(), fmt.Errorf("decode at step %d: %w", i, err)
		}
	}

	return output.String(), nil
}

// tokenize delegates to the model's tokenizer.
func (c *Context) tokenize(text string) ([]int32, error) {
	return c.model.Tokenize(text, true, false)
}

// getLogitsIth returns logits for the i-th output token, falling back to
// getLogits if llama_get_logits_ith is not available.
func (c *Context) getLogitsIth(i int32) *float32 {
	if LlamaGetLogitsIth != nil {
		return LlamaGetLogitsIth(c.handle, i)
	}
	if LlamaGetLogits != nil {
		return LlamaGetLogits(c.handle)
	}
	return nil
}

// sampleToken picks the next token from logits (argmax with temperature).
func sampleToken(logits []float32, temp float32, topK int32, lastTokens []int32, repeatPenalty float32) int32 {
	n := len(logits)

	// Apply temperature
	if temp > 0 && temp != 1.0 {
		for i := 0; i < n; i++ {
			logits[i] /= temp
		}
	}

	// Apply repetition penalty
	if repeatPenalty != 1.0 && len(lastTokens) > 0 {
		for _, tok := range lastTokens {
			if tok >= 0 && int(tok) < n {
				if logits[tok] < 0 {
					logits[tok] *= repeatPenalty
				} else {
					logits[tok] /= repeatPenalty
				}
			}
		}
	}

	// Argmax
	best := int32(0)
	bestVal := logits[0]
	for i := 1; i < n; i++ {
		if logits[i] > bestVal {
			bestVal = logits[i]
			best = int32(i)
		}
	}
	return best
}

// buildBatch returns a CBatch for the given tokens.
// If logitsLast is true, only the final token requests logit output.
func buildBatch(tokens []int32, logitsLast bool) CBatch {
	var nSeqID int32 = 1
	seqIDs := make([]*int32, len(tokens))
	for i := range seqIDs {
		var sid int32 = 0
		seqIDs[i] = &sid
	}
	var logitsArr []int8
	if logitsLast {
		logitsArr = make([]int8, len(tokens))
		logitsArr[len(tokens)-1] = 1
	}
	return CBatch{
		NTokens: int32(len(tokens)),
		Token:   &tokens[0],
		Embd:    nil,
		Pos:     nil,
		NSeqId:  &nSeqID,
		SeqId:   &seqIDs[0],
		Logits:  ptrOrNil(logitsArr),
	}
}

// buildSingleTokenBatch returns a CBatch for a single token at the given position.
func buildSingleTokenBatch(token, pos int32) CBatch {
	var nSeqID int32 = 1
	var seqID int32 = 0
	seqIDs := []*int32{&seqID}
	var wantLogits int8 = 1
	return CBatch{
		NTokens: 1,
		Token:   &token,
		Embd:    nil,
		Pos:     &pos,
		NSeqId:  &nSeqID,
		SeqId:   &seqIDs[0],
		Logits:  &wantLogits,
	}
}

// ptrOrNil returns a pointer to the first element of s, or nil if s is empty.
func ptrOrNil[T any](s []T) *T {
	if len(s) == 0 {
		return nil
	}
	return &s[0]
}

// checkStopSequences returns true if s contains any of the stop strings.
func checkStopSequences(s string, stops []string) bool {
	for _, stop := range stops {
		if stop != "" && strings.Contains(s, stop) {
			return true
		}
	}
	return false
}

// NCtx returns the context size.
func (c *Context) NCtx() uint32 {
	if LlamaNCtx != nil {
		return LlamaNCtx(c.handle)
	}
	return 0
}
