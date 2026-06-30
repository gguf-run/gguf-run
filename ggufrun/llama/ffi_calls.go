package llama

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// ffiType mirrors the C ffi_type struct from libffi.
type ffiType struct {
	Size      uint64
	Alignment uint16
	Typ       uint16
	Elements  unsafe.Pointer // **ffi_type, nil-terminated array for struct types
}

// ffiCif mirrors the first 6 fields of the C ffi_cif struct.
type ffiCif struct {
	Abi      uint32
	NArgs    uint32
	ArgTypes unsafe.Pointer // **ffi_type
	RType    unsafe.Pointer // *ffi_type
	Bytes    uint32
	Flags    uint32
}

// CModelParams matches the C struct llama_model_params from b9564.
type CModelParams struct {
	Devices               uintptr
	TensorBuftOverrides   uintptr
	NGpuLayers            int32
	SplitMode             int32
	MainGpu               int32
	TensorSplit           uintptr
	ProgressCallback      uintptr
	ProgressCallbackUserData uintptr
	KVOverrides           uintptr
	VocabOnly             bool
	UseMmap               bool
	UseDirectIo           bool
	UseMlock              bool
	CheckTensors          bool
	UseExtraBufts         bool
	NoHost                bool
	NoAlloc               bool
}

// CContextParams matches the C struct llama_context_params from b9564.
type CContextParams struct {
	NCtx           uint32
	NBatch         uint32
	NUBatch        uint32
	NSeqMax        uint32
	NRSSeq         uint32
	NOutputsMax    uint32
	NThreads       int32
	NThreadsBatch  int32
	CtxType        int32
	RopeScalingType int32
	PoolingType    int32
	AttentionType  int32
	FlashAttnType  int32
	RopeFreqBase   float32
	RopeFreqScale  float32
	YarnExtFactor  float32
	YarnAttnFactor float32
	YarnBetaFast   float32
	YarnBetaSlow   float32
	YarnOrigCtx    uint32
	DefragThold    float32
	CbEval         uintptr
	CbEvalUserData uintptr
	TypeK          int32
	TypeV          int32
	AbortCallback  uintptr
	AbortCallbackData uintptr
	Embeddings     bool
	OffloadKQV     bool
	NoPerf         bool
	OpOffload      bool
	SwaFull        bool
	KvUnified      bool
	Samplers       uintptr
	NSamplers      uintptr
	CtxOther       uintptr
}

const (
	ffiTypeVoid = iota
	ffiTypeInt
	ffiTypeFloat
	ffiTypeDouble
	ffiTypeLongdouble
	ffiTypeUint8
	ffiTypeSint8
	ffiTypeUint16
	ffiTypeSint16
	ffiTypeUint32
	ffiTypeSint32
	ffiTypeUint64
	ffiTypeSint64
	ffiTypeStruct
	ffiTypePointer
	ffiTypeComplex
)

// Predefined ffi_type pointers loaded from libffi.
var (
	ffiTypeVoidPtr    *ffiType
	ffiTypeU8Ptr      *ffiType
	ffiTypeS32Ptr     *ffiType
	ffiTypeU32Ptr     *ffiType
	ffiTypeFloatPtr   *ffiType
	ffiTypePointerPtr *ffiType
)

// Composite ffi_type values for llama C structs.
var (
	cModelParamsType   ffiType
	cContextParamsType ffiType
	cBatchType         ffiType
	initTypesOnce      sync.Once
)

// Stable backing arrays for composite type element lists (keep GC from
// collecting them since ffi_type.Elements points into slice backing store).
var (
	modelParamElems   []*ffiType
	contextParamElems []*ffiType
	batchElems        []*ffiType
)

func initTypes() {
	initTypesOnce.Do(func() {
		modelParamElems = []*ffiType{
			ffiTypePointerPtr, ffiTypePointerPtr,
			ffiTypeS32Ptr, ffiTypeS32Ptr, ffiTypeS32Ptr,
			ffiTypePointerPtr, ffiTypePointerPtr, ffiTypePointerPtr, ffiTypePointerPtr,
			ffiTypeU8Ptr, ffiTypeU8Ptr, ffiTypeU8Ptr, ffiTypeU8Ptr,
			ffiTypeU8Ptr, ffiTypeU8Ptr, ffiTypeU8Ptr, ffiTypeU8Ptr,
			nil,
		}
		cModelParamsType = ffiType{
			Typ:      ffiTypeStruct,
			Elements: unsafe.Pointer(&modelParamElems[0]),
		}

		contextParamElems = []*ffiType{
			ffiTypeU32Ptr, ffiTypeU32Ptr, ffiTypeU32Ptr, ffiTypeU32Ptr,
			ffiTypeU32Ptr, ffiTypeU32Ptr,
			ffiTypeS32Ptr, ffiTypeS32Ptr,
			ffiTypeS32Ptr, ffiTypeS32Ptr, ffiTypeS32Ptr, ffiTypeS32Ptr, ffiTypeS32Ptr,
			ffiTypeFloatPtr, ffiTypeFloatPtr, ffiTypeFloatPtr, ffiTypeFloatPtr,
			ffiTypeFloatPtr, ffiTypeFloatPtr,
			ffiTypeU32Ptr, ffiTypeFloatPtr,
			ffiTypePointerPtr, ffiTypePointerPtr,
			ffiTypeS32Ptr, ffiTypeS32Ptr,
			ffiTypePointerPtr, ffiTypePointerPtr,
			ffiTypeU8Ptr, ffiTypeU8Ptr, ffiTypeU8Ptr, ffiTypeU8Ptr,
			ffiTypeU8Ptr, ffiTypeU8Ptr,
			ffiTypePointerPtr, ffiTypePointerPtr, ffiTypePointerPtr,
			nil,
		}
		cContextParamsType = ffiType{
			Typ:      ffiTypeStruct,
			Elements: unsafe.Pointer(&contextParamElems[0]),
		}

		batchElems = []*ffiType{
			ffiTypeS32Ptr,
			ffiTypePointerPtr,
			ffiTypePointerPtr,
			ffiTypePointerPtr,
			ffiTypePointerPtr,
			ffiTypePointerPtr,
			ffiTypePointerPtr,
			nil,
		}
		cBatchType = ffiType{
			Typ:      ffiTypeStruct,
			Elements: unsafe.Pointer(&batchElems[0]),
		}
	})
}

// Libffi function addresses loaded at runtime.
var (
	prepCifFn uintptr
	callFn    uintptr
)

// ffiABI returns the FFI_DEFAULT_ABI value for the current platform.
func ffiABI() uint32 {
	switch runtime.GOARCH {
	case "arm64":
		return 1
	default:
		return 2
	}
}

// loadLibffi errors
var (
	libffiLoadOnce sync.Once
	libffiLoadErr  error
)

func loadLibffi() error {
	libffiLoadOnce.Do(func() {
		handle, err := purego.Dlopen("libffi.so.8", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			libffiLoadErr = fmt.Errorf("libffi.so.8 not found: %w\nInstall with: apt install libffi8  (Debian/Ubuntu)\n          or: apk add libffi       (Alpine)\n          or: pacman -S libffi     (Arch)", err)
			return
		}

		prepCifFn, err = purego.Dlsym(handle, "ffi_prep_cif")
		if err != nil {
			libffiLoadErr = fmt.Errorf("ffi_prep_cif symbol not found: %w", err)
			return
		}

		callFn, err = purego.Dlsym(handle, "ffi_call")
		if err != nil {
			libffiLoadErr = fmt.Errorf("ffi_call symbol not found: %w", err)
			return
		}

		typePtrs := []struct {
			name string
			dst  **ffiType
		}{
			{"ffi_type_void", &ffiTypeVoidPtr},
			{"ffi_type_uint8", &ffiTypeU8Ptr},
			{"ffi_type_sint32", &ffiTypeS32Ptr},
			{"ffi_type_uint32", &ffiTypeU32Ptr},
			{"ffi_type_float", &ffiTypeFloatPtr},
			{"ffi_type_pointer", &ffiTypePointerPtr},
		}
		for _, tp := range typePtrs {
			addr, err := purego.Dlsym(handle, tp.name)
			if err != nil {
				libffiLoadErr = fmt.Errorf("%s symbol not found: %w", tp.name, err)
				return
			}
			*tp.dst = *(**ffiType)(unsafe.Pointer(&addr))
		}

		initTypes()
	})
	return libffiLoadErr
}

// LoadModelFFI calls llama_model_load_from_file with correct struct-by-value ABI.
func LoadModelFFI(path *byte, params *CModelParams) (uintptr, error) {
	if LlamaModelLoadFromFileAddr == 0 {
		return 0, fmt.Errorf("llama_model_load_from_file not available")
	}
	if err := loadLibffi(); err != nil {
		return 0, fmt.Errorf("libffi: %w", err)
	}

	var cif ffiCif
	atypes := [2]*ffiType{ffiTypePointerPtr, &cModelParamsType}
	ret, _, _ := purego.SyscallN(prepCifFn,
		uintptr(unsafe.Pointer(&cif)),
		uintptr(ffiABI()),
		2,
		uintptr(unsafe.Pointer(ffiTypePointerPtr)),
		uintptr(unsafe.Pointer(&atypes[0])),
	)
	if ret != 0 {
		return 0, fmt.Errorf("ffi_prep_cif failed with status %d", ret)
	}

	var result uintptr
	pathPtr := unsafe.Pointer(path)
	values := [2]unsafe.Pointer{
		unsafe.Pointer(&pathPtr),
		unsafe.Pointer(params),
	}
	purego.SyscallN(callFn,
		uintptr(unsafe.Pointer(&cif)),
		LlamaModelLoadFromFileAddr,
		uintptr(unsafe.Pointer(&result)),
		uintptr(unsafe.Pointer(&values[0])),
	)
	if result == 0 {
		return 0, fmt.Errorf("llama_model_load_from_file returned NULL")
	}
	return result, nil
}

// InitFromModelFFI calls llama_init_from_model with correct struct-by-value ABI.
func InitFromModelFFI(model uintptr, params *CContextParams) (uintptr, error) {
	if LlamaInitFromModelAddr == 0 {
		return 0, fmt.Errorf("llama_init_from_model not available")
	}
	if err := loadLibffi(); err != nil {
		return 0, fmt.Errorf("libffi: %w", err)
	}

	var cif ffiCif
	atypes := [2]*ffiType{ffiTypePointerPtr, &cContextParamsType}
	ret, _, _ := purego.SyscallN(prepCifFn,
		uintptr(unsafe.Pointer(&cif)),
		uintptr(ffiABI()),
		2,
		uintptr(unsafe.Pointer(ffiTypePointerPtr)),
		uintptr(unsafe.Pointer(&atypes[0])),
	)
	if ret != 0 {
		return 0, fmt.Errorf("ffi_prep_cif failed with status %d", ret)
	}

	var result uintptr
	modelPtr := model
	values := [2]unsafe.Pointer{
		unsafe.Pointer(&modelPtr),
		unsafe.Pointer(params),
	}
	purego.SyscallN(callFn,
		uintptr(unsafe.Pointer(&cif)),
		LlamaInitFromModelAddr,
		uintptr(unsafe.Pointer(&result)),
		uintptr(unsafe.Pointer(&values[0])),
	)
	if result == 0 {
		return 0, fmt.Errorf("llama_init_from_model returned NULL")
	}
	return result, nil
}

// DecodeFFI calls llama_decode with batch passed by value via FFI.
func DecodeFFI(ctx uintptr, batch *CBatch) (int32, error) {
	if LlamaDecodeAddr == 0 {
		return -1, fmt.Errorf("llama_decode not available")
	}
	if err := loadLibffi(); err != nil {
		return -1, fmt.Errorf("libffi: %w", err)
	}

	var cif ffiCif
	atypes := [2]*ffiType{ffiTypePointerPtr, &cBatchType}
	ret, _, _ := purego.SyscallN(prepCifFn,
		uintptr(unsafe.Pointer(&cif)),
		uintptr(ffiABI()),
		2,
		uintptr(unsafe.Pointer(ffiTypeS32Ptr)),
		uintptr(unsafe.Pointer(&atypes[0])),
	)
	if ret != 0 {
		return -1, fmt.Errorf("ffi_prep_cif failed with status %d", ret)
	}

	var result int32
	ctxPtr := ctx
	values := [2]unsafe.Pointer{
		unsafe.Pointer(&ctxPtr),
		unsafe.Pointer(batch),
	}
	purego.SyscallN(callFn,
		uintptr(unsafe.Pointer(&cif)),
		LlamaDecodeAddr,
		uintptr(unsafe.Pointer(&result)),
		uintptr(unsafe.Pointer(&values[0])),
	)
	if result != 0 {
		return result, fmt.Errorf("llama_decode failed with code %d", result)
	}
	return 0, nil
}
