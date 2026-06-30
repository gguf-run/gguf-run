package llama

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/jupiterrider/ffi"
)

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

var (
	modelParamsType   ffi.Type
	contextParamsType ffi.Type
	cifModelLoad      ffi.Cif
	cifContextCreate  ffi.Cif
	onceModelLoad     sync.Once
	onceContextCreate sync.Once
)

func initModelLoadCIF() {
	onceModelLoad.Do(func() {
		modelParamsType = ffi.NewType(
			&ffi.TypePointer,
			&ffi.TypePointer,
			&ffi.TypeSint32,
			&ffi.TypeSint32,
			&ffi.TypeSint32,
			&ffi.TypePointer,
			&ffi.TypePointer,
			&ffi.TypePointer,
			&ffi.TypePointer,
			&ffi.TypeUint8,
			&ffi.TypeUint8,
			&ffi.TypeUint8,
			&ffi.TypeUint8,
			&ffi.TypeUint8,
			&ffi.TypeUint8,
			&ffi.TypeUint8,
			&ffi.TypeUint8,
		)
		ffi.PrepCif(&cifModelLoad, ffi.DefaultAbi, 2, &ffi.TypePointer, &ffi.TypePointer, &modelParamsType)
	})
}

func initContextCreateCIF() {
	onceContextCreate.Do(func() {
		contextParamsType = ffi.NewType(
			&ffi.TypeUint32, &ffi.TypeUint32, &ffi.TypeUint32, &ffi.TypeUint32,
			&ffi.TypeUint32, &ffi.TypeUint32, &ffi.TypeSint32, &ffi.TypeSint32,
			&ffi.TypeSint32, &ffi.TypeSint32, &ffi.TypeSint32, &ffi.TypeSint32,
			&ffi.TypeSint32,
			&ffi.TypeFloat, &ffi.TypeFloat, &ffi.TypeFloat, &ffi.TypeFloat,
			&ffi.TypeFloat, &ffi.TypeFloat, &ffi.TypeUint32, &ffi.TypeFloat,
			&ffi.TypePointer, &ffi.TypePointer,
			&ffi.TypeSint32, &ffi.TypeSint32,
			&ffi.TypePointer, &ffi.TypePointer,
			&ffi.TypeUint8, &ffi.TypeUint8, &ffi.TypeUint8, &ffi.TypeUint8,
			&ffi.TypeUint8, &ffi.TypeUint8,
			&ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer,
		)
		ffi.PrepCif(&cifContextCreate, ffi.DefaultAbi, 2, &ffi.TypePointer, &ffi.TypePointer, &contextParamsType)
	})
}

// LoadModelFFI calls llama_model_load_from_file with correct struct-by-value ABI.
func LoadModelFFI(path *byte, params *CModelParams) (uintptr, error) {
	if LlamaModelLoadFromFileAddr == 0 {
		return 0, fmt.Errorf("llama_model_load_from_file not available")
	}
	initModelLoadCIF()

	var ret uintptr
	pathPtr := unsafe.Pointer(path)
	paramsPtr := unsafe.Pointer(params)
	retPtr := unsafe.Pointer(&ret)
	ffi.Call(&cifModelLoad, LlamaModelLoadFromFileAddr, retPtr,
		unsafe.Pointer(&pathPtr),
		paramsPtr,
	)
	if ret == 0 {
		return 0, fmt.Errorf("llama_model_load_from_file returned NULL")
	}
	return ret, nil
}

// InitFromModelFFI calls llama_init_from_model with correct struct-by-value ABI.
func InitFromModelFFI(model uintptr, params *CContextParams) (uintptr, error) {
	if LlamaInitFromModelAddr == 0 {
		return 0, fmt.Errorf("llama_init_from_model not available")
	}
	initContextCreateCIF()

	var ret uintptr
	paramsArg := unsafe.Pointer(params)
	ffi.Call(&cifContextCreate, LlamaInitFromModelAddr, unsafe.Pointer(&ret),
		unsafe.Pointer(&model),
		paramsArg,
	)
	if ret == 0 {
		return 0, fmt.Errorf("llama_init_from_model returned NULL")
	}
	return ret, nil
}
