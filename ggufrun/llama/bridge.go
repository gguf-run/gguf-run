package llama

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// C-compatible struct for llama_batch (matching b6099 layout)
type CBatch struct {
	NTokens int32
	Token   *int32
	Embd    *float32
	Pos     *int32
	NSeqId  *int32
	SeqId   **int32
	Logits  *int8
}

// Size checking at compile time
var _ = unsafe.Sizeof(CBatch{})

// C API function pointers - populated by RegisterFunctions
var (
	// Model management (struct params - uses ffi)
	LlamaModelLoadFromFile func(path *byte, params unsafe.Pointer) uintptr

	// Model management (simple params - uses purego)
	LlamaModelFree  func(model uintptr)
	LlamaModelDesc  func(model uintptr, buf *byte, bufSize uint64) int32
	LlamaModelSize  func(model uintptr) uint64
	LlamaModelNVocab func(model uintptr) int32
	LlamaModelNEmbd func(model uintptr) int32
	LlamaModelNLayer func(model uintptr) int32

	// Context management (struct params - uses ffi)
	LlamaInitFromModel func(model uintptr, params unsafe.Pointer) uintptr

	// Context management (simple params - uses purego)
	LlamaFree          func(ctx uintptr)
	LlamaNCtx          func(ctx uintptr) uint32
	LlamaNBatch        func(ctx uintptr) uint32
	LlamaNSeqMax       func(ctx uintptr) uint32
	LlamaGetModel      func(ctx uintptr) uintptr

	// Tokenization (uses llama_vocab in b6099, but deprecated wrappers take model)
	LlamaTokenize       func(model uintptr, text *byte, textLen int32, tokens *int32, nTokens int32, addBos bool, special bool) int32
	LlamaTokenToPiece   func(model uintptr, token int32, buf *byte, length int32, lstrip int32, special bool) int32
	LlamaDetokenize     func(ctx uintptr, tokens *int32, nTokens int32, text *byte, textLenMax int32, removeSpecials bool, unparseSpecials bool) int32

	// Special tokens (deprecated wrappers on model) - use purego
	LlamaTokenBos func(model uintptr) int32
	LlamaTokenEos func(model uintptr) int32
	LlamaTokenNl  func(model uintptr) int32
	LlamaTokenEot func(model uintptr) int32
	LlamaTokenSep func(model uintptr) int32
	LlamaTokenPad func(model uintptr) int32
	LlamaTokenMask func(model uintptr) int32

	// Inference (struct params - uses ffi)
	LlamaDecode    func(ctx uintptr, batch unsafe.Pointer) int32
	LlamaEncode    func(ctx uintptr, batch unsafe.Pointer) int32
	LlamaBatchGetOne func(tokens *int32, nTokens int32) unsafe.Pointer
	LlamaBatchFree func(batch unsafe.Pointer)

	// Sampling (simple params - uses purego)
	LlamaSampleTemperature            func(ctx uintptr, token int32, temp float32) int32
	LlamaSampleTopK                   func(ctx uintptr, token int32, k int32) int32
	LlamaSampleTopP                   func(ctx uintptr, token int32, p float32, minKeep int32) int32
	LlamaSampleMinP                   func(ctx uintptr, token int32, p float32) int32
	LlamaSampleTypical                func(ctx uintptr, token int32, p float32, minKeep int32) int32
	LlamaSampleRepetitionPenalty      func(ctx uintptr, token int32, penalty float32, lastTokens *int32, lastTokensSize int32) int32
	LlamaSampleFrequencyPenalty       func(ctx uintptr, token int32, penalty float32, lastTokens *int32, lastTokensSize int32) int32
	LlamaSamplePresencePenalty        func(ctx uintptr, token int32, penalty float32, lastTokens *int32, lastTokensSize int32) int32

	// Logits
	LlamaGetLogits    func(ctx uintptr) *float32
	LlamaGetLogitsIth func(ctx uintptr, i int32) *float32

	// Backend
	LlamaBackendInit func()
	LlamaBackendFree func()
	LlamaSupportsMMap func() bool
	LlamaSupportsMLock func() bool
	LlamaSupportsGPUOffload func() bool
)

// Sizes of C structs for allocation
const (
	SizeofLlamaModelParams   = 80
	SizeofLlamaContextParams = 160
	SizeofLlamaBatch         = 48
)

// RegisterFunctions binds all C API functions from the loaded library.
func RegisterFunctions() error {
	handle := Handle()
	if handle == 0 {
		return ErrLibraryNotLoaded
	}

	// Model management (with struct params - need ffi)
	// TODO: use ffi for proper struct-by-value support
	// For now, register as if taking pointer - will crash at runtime for inference
	purego.RegisterLibFunc(&LlamaModelLoadFromFile, handle, "llama_model_load_from_file")
	purego.RegisterLibFunc(&LlamaInitFromModel, handle, "llama_init_from_model")
	purego.RegisterLibFunc(&LlamaDecode, handle, "llama_decode")
	purego.RegisterLibFunc(&LlamaEncode, handle, "llama_encode")
	purego.RegisterLibFunc(&LlamaBatchGetOne, handle, "llama_batch_get_one")
	purego.RegisterLibFunc(&LlamaBatchFree, handle, "llama_batch_free")

	// Model management (simple params)
	purego.RegisterLibFunc(&LlamaModelFree, handle, "llama_model_free")
	purego.RegisterLibFunc(&LlamaModelDesc, handle, "llama_model_desc")
	purego.RegisterLibFunc(&LlamaModelSize, handle, "llama_model_size")
	purego.RegisterLibFunc(&LlamaModelNVocab, handle, "llama_model_n_vocab")
	purego.RegisterLibFunc(&LlamaModelNEmbd, handle, "llama_model_n_embd")
	purego.RegisterLibFunc(&LlamaModelNLayer, handle, "llama_model_n_layer")

	// Context management (simple params)
	purego.RegisterLibFunc(&LlamaFree, handle, "llama_free")
	purego.RegisterLibFunc(&LlamaNCtx, handle, "llama_n_ctx")
	purego.RegisterLibFunc(&LlamaNBatch, handle, "llama_n_batch")
	purego.RegisterLibFunc(&LlamaNSeqMax, handle, "llama_n_seq_max")
	purego.RegisterLibFunc(&LlamaGetModel, handle, "llama_get_model")

	// Tokenization
	purego.RegisterLibFunc(&LlamaTokenize, handle, "llama_tokenize")
	purego.RegisterLibFunc(&LlamaTokenToPiece, handle, "llama_token_to_piece")
	purego.RegisterLibFunc(&LlamaDetokenize, handle, "llama_detokenize")

	// Special tokens
	purego.RegisterLibFunc(&LlamaTokenBos, handle, "llama_token_bos")
	purego.RegisterLibFunc(&LlamaTokenEos, handle, "llama_token_eos")
	purego.RegisterLibFunc(&LlamaTokenNl, handle, "llama_token_nl")
	purego.RegisterLibFunc(&LlamaTokenEot, handle, "llama_token_eot")
	purego.RegisterLibFunc(&LlamaTokenSep, handle, "llama_token_sep")
	purego.RegisterLibFunc(&LlamaTokenPad, handle, "llama_token_pad")
	purego.RegisterLibFunc(&LlamaTokenMask, handle, "llama_token_mask")

	// Sampling
	purego.RegisterLibFunc(&LlamaSampleTemperature, handle, "llama_sample_temperature")
	purego.RegisterLibFunc(&LlamaSampleTopK, handle, "llama_sample_top_k")
	purego.RegisterLibFunc(&LlamaSampleTopP, handle, "llama_sample_top_p")
	purego.RegisterLibFunc(&LlamaSampleMinP, handle, "llama_sample_min_p")
	purego.RegisterLibFunc(&LlamaSampleTypical, handle, "llama_sample_typical")
	purego.RegisterLibFunc(&LlamaSampleRepetitionPenalty, handle, "llama_sample_repetition_penalty")
	purego.RegisterLibFunc(&LlamaSampleFrequencyPenalty, handle, "llama_sample_frequency_penalty")
	purego.RegisterLibFunc(&LlamaSamplePresencePenalty, handle, "llama_sample_presence_penalty")

	// Logits
	purego.RegisterLibFunc(&LlamaGetLogits, handle, "llama_get_logits")
	purego.RegisterLibFunc(&LlamaGetLogitsIth, handle, "llama_get_logits_ith")

	// Backend
	purego.RegisterLibFunc(&LlamaBackendInit, handle, "llama_backend_init")
	purego.RegisterLibFunc(&LlamaBackendFree, handle, "llama_backend_free")
	purego.RegisterLibFunc(&LlamaSupportsMMap, handle, "llama_supports_mmap")
	purego.RegisterLibFunc(&LlamaSupportsMLock, handle, "llama_supports_mlock")
	purego.RegisterLibFunc(&LlamaSupportsGPUOffload, handle, "llama_supports_gpu_offload")

	return nil
}

// MustRegisterFunctions calls RegisterFunctions and panics on error.
func MustRegisterFunctions() {
	if err := RegisterFunctions(); err != nil {
		panic("gguf-run: failed to register llama.cpp functions: " + err.Error())
	}
}
