package llama

import (
	"fmt"
	"unsafe"

	"github.com/ebitengine/purego"
)

// C-compatible struct for llama_batch (matching b6099 layout).
type CBatch struct {
	NTokens int32
	Token   *int32
	Embd    *float32
	Pos     *int32
	NSeqId  *int32
	SeqId   **int32
	Logits  *int8
}


// C API function pointers — populated by RegisterFunctions.
// Optional functions may be nil if the loaded library doesn't export them.
var (
	// Raw function addresses for FFI struct-by-value calls
	LlamaModelLoadFromFileAddr uintptr
	LlamaInitFromModelAddr     uintptr
	LlamaDecodeAddr            uintptr

	// Model management
	LlamaModelLoadFromFile func(path *byte, params unsafe.Pointer) uintptr
	LlamaModelFree         func(model uintptr)
	LlamaModelDesc         func(model uintptr, buf *byte, bufSize uint64) int32
	LlamaModelSize         func(model uintptr) uint64
	LlamaModelNVocab       func(model uintptr) int32
	LlamaModelNEmbd        func(model uintptr) int32
	LlamaModelNLayer       func(model uintptr) int32

	// Context management
	LlamaInitFromModel func(model uintptr, params unsafe.Pointer) uintptr
	LlamaFree          func(ctx uintptr)
	LlamaNCtx          func(ctx uintptr) uint32
	LlamaNBatch        func(ctx uintptr) uint32
	LlamaNSeqMax       func(ctx uintptr) uint32
	LlamaGetModel      func(ctx uintptr) uintptr

	// Tokenization
	LlamaTokenize     func(model uintptr, text *byte, textLen int32, tokens *int32, nTokens int32, addBos bool, special bool) int32
	LlamaTokenToPiece func(model uintptr, token int32, buf *byte, length int32, lstrip int32, special bool) int32
	LlamaDetokenize   func(ctx uintptr, tokens *int32, nTokens int32, text *byte, textLenMax int32, removeSpecials bool, unparseSpecials bool) int32

	// Special tokens
	LlamaTokenBos  func(model uintptr) int32
	LlamaTokenEos  func(model uintptr) int32
	LlamaTokenNl   func(model uintptr) int32
	LlamaTokenEot  func(model uintptr) int32
	LlamaTokenSep  func(model uintptr) int32
	LlamaTokenPad  func(model uintptr) int32
	LlamaTokenMask func(model uintptr) int32

	// Inference (struct params)
	LlamaDecode      func(ctx uintptr, batch unsafe.Pointer) int32
	LlamaEncode      func(ctx uintptr, batch unsafe.Pointer) int32
	LlamaBatchGetOne func(tokens *int32, nTokens int32) unsafe.Pointer
	LlamaBatchFree   func(batch unsafe.Pointer)

	// Sampling
	LlamaSampleTemperature       func(ctx uintptr, token int32, temp float32) int32
	LlamaSampleTopK              func(ctx uintptr, token int32, k int32) int32
	LlamaSampleTopP              func(ctx uintptr, token int32, p float32, minKeep int32) int32
	LlamaSampleMinP              func(ctx uintptr, token int32, p float32) int32
	LlamaSampleTypical           func(ctx uintptr, token int32, p float32, minKeep int32) int32
	LlamaSampleRepetitionPenalty func(ctx uintptr, token int32, penalty float32, lastTokens *int32, lastTokensSize int32) int32
	LlamaSampleFrequencyPenalty  func(ctx uintptr, token int32, penalty float32, lastTokens *int32, lastTokensSize int32) int32
	LlamaSamplePresencePenalty   func(ctx uintptr, token int32, penalty float32, lastTokens *int32, lastTokensSize int32) int32

	// Logits
	LlamaGetLogits    func(ctx uintptr) *float32
	LlamaGetLogitsIth func(ctx uintptr, i int32) *float32

	// Backend
	LlamaBackendInit       func()
	LlamaBackendFree       func()
	LlamaSupportsMMap      func() bool
	LlamaSupportsMLock     func() bool
	LlamaSupportsGPUOffload func() bool
)

// Sizes of C structs for allocation.
const (
	SizeofLlamaModelParams   = 80
	SizeofLlamaContextParams = 160
	SizeofLlamaBatch         = 48
)

// tryRegister binds a function pointer to a named symbol, returning false if
// the symbol is not found (instead of panicking like purego.RegisterLibFunc).
// Returns the raw symbol address and whether the symbol was found.
func tryRegister(handle uintptr, fnPtr any, names ...string) (uintptr, bool) {
	for _, name := range names {
		addr, err := purego.Dlsym(handle, name)
		if err != nil {
			continue
		}
		purego.RegisterLibFunc(fnPtr, handle, name)
		return addr, true
	}
	return 0, false
}

// RegisterFunctions binds all C API functions from the loaded library.
// Missing symbols are silently skipped — callers must check function pointers
// before calling.
func RegisterFunctions() error {
	handle := Handle()
	if handle == 0 {
		return ErrLibraryNotLoaded
	}

	// Model management
	LlamaModelLoadFromFileAddr, _ = tryRegister(handle, &LlamaModelLoadFromFile, "llama_model_load_from_file", "llama_load_model_from_file")
	tryRegister(handle, &LlamaModelFree, "llama_model_free", "llama_free_model")
	tryRegister(handle, &LlamaModelDesc, "llama_model_desc")
	tryRegister(handle, &LlamaModelSize, "llama_model_size")
	tryRegister(handle, &LlamaModelNVocab, "llama_model_n_vocab", "llama_n_vocab")
	tryRegister(handle, &LlamaModelNEmbd, "llama_model_n_embd", "llama_n_embd")
	tryRegister(handle, &LlamaModelNLayer, "llama_model_n_layer", "llama_n_layer")

	// Context management
	LlamaInitFromModelAddr, _ = tryRegister(handle, &LlamaInitFromModel, "llama_init_from_model", "llama_new_context_with_model")
	tryRegister(handle, &LlamaFree, "llama_free")
	tryRegister(handle, &LlamaNCtx, "llama_n_ctx")
	tryRegister(handle, &LlamaNBatch, "llama_n_batch")
	tryRegister(handle, &LlamaNSeqMax, "llama_n_seq_max")
	tryRegister(handle, &LlamaGetModel, "llama_get_model")

	// Tokenization
	tryRegister(handle, &LlamaTokenize, "llama_tokenize")
	tryRegister(handle, &LlamaTokenToPiece, "llama_token_to_piece")
	tryRegister(handle, &LlamaDetokenize, "llama_detokenize")

	// Special tokens — try new vocab_ names first, then old token_ names
	tryRegister(handle, &LlamaTokenBos, "llama_vocab_bos", "llama_token_bos")
	tryRegister(handle, &LlamaTokenEos, "llama_vocab_eos", "llama_token_eos")
	tryRegister(handle, &LlamaTokenNl, "llama_vocab_nl", "llama_token_nl")
	tryRegister(handle, &LlamaTokenEot, "llama_vocab_eot", "llama_token_eot")
	tryRegister(handle, &LlamaTokenSep, "llama_vocab_sep", "llama_token_sep")
	tryRegister(handle, &LlamaTokenPad, "llama_vocab_pad", "llama_token_pad")
	tryRegister(handle, &LlamaTokenMask, "llama_vocab_mask", "llama_token_mask")

	// Inference
	LlamaDecodeAddr, _ = tryRegister(handle, &LlamaDecode, "llama_decode")
	tryRegister(handle, &LlamaEncode, "llama_encode")
	tryRegister(handle, &LlamaBatchGetOne, "llama_batch_get_one")
	tryRegister(handle, &LlamaBatchFree, "llama_batch_free")

	// Sampling
	tryRegister(handle, &LlamaSampleTemperature, "llama_sample_temperature")
	tryRegister(handle, &LlamaSampleTopK, "llama_sample_top_k")
	tryRegister(handle, &LlamaSampleTopP, "llama_sample_top_p")
	tryRegister(handle, &LlamaSampleMinP, "llama_sample_min_p")
	tryRegister(handle, &LlamaSampleTypical, "llama_sample_typical")
	tryRegister(handle, &LlamaSampleRepetitionPenalty, "llama_sample_repetition_penalty")
	tryRegister(handle, &LlamaSampleFrequencyPenalty, "llama_sample_frequency_penalty")
	tryRegister(handle, &LlamaSamplePresencePenalty, "llama_sample_presence_penalty")

	// Logits
	tryRegister(handle, &LlamaGetLogits, "llama_get_logits")
	tryRegister(handle, &LlamaGetLogitsIth, "llama_get_logits_ith")

	// Backend
	tryRegister(handle, &LlamaBackendInit, "llama_backend_init")
	tryRegister(handle, &LlamaBackendFree, "llama_backend_free")
	tryRegister(handle, &LlamaSupportsMMap, "llama_supports_mmap")
	tryRegister(handle, &LlamaSupportsMLock, "llama_supports_mlock")
	tryRegister(handle, &LlamaSupportsGPUOffload, "llama_supports_gpu_offload")

	return nil
}

// MustRegisterFunctions calls RegisterFunctions and panics on error.
func MustRegisterFunctions() {
	if err := RegisterFunctions(); err != nil {
		panic("gguf-run: failed to register llama.cpp functions: " + err.Error())
	}
}

// checkRequired panics if any of the listed function pointers is nil.
func checkRequired(fns ...any) {
	for _, fn := range fns {
		// Use fmt.Sprintf to check for nil interface
		if s := fmt.Sprintf("%v", fn); s == "<nil>" {
			panic("gguf-run: required llama.cpp function not available in loaded library")
		}
	}
}
