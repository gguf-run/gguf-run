package llama

import "fmt"

// GenerateOptions contains options for text generation.
type GenerateOptions struct {
	Prompt          string
	MaxTokens       int32
	Temperature     float32
	TopK            int32
	TopP            float32
	MinP            float32
	TypicalP        float32
	RepeatPenalty   float32
	FreqPenalty     float32
	PresencePenalty float32
	Seed            int32
	StopSequences   []string
	IgnoreEOS       bool
	Stream          bool
	Callback        func(string)
}

// ModelInfo contains model metadata.
type ModelInfo struct {
	Name        string
	NVocab      int32
	NEmbd       int32
	NLayers     int32
	SizeBytes   int64
	Description string
}

// ErrLibraryNotLoaded is returned when llama.cpp library is not loaded.
var ErrLibraryNotLoaded = fmt.Errorf("llama.cpp library not loaded: call llama.Init() first")

// DefaultGenerateOptions returns sensible defaults for generation.
func DefaultGenerateOptions() GenerateOptions {
	return GenerateOptions{
		MaxTokens:       0,
		Temperature:     0.8,
		TopK:            40,
		TopP:            0.95,
		MinP:            0.05,
		TypicalP:        1.0,
		RepeatPenalty:   1.1,
		FreqPenalty:     0.0,
		PresencePenalty: 0.0,
		Seed:            0,
		IgnoreEOS:       false,
		Stream:          false,
	}
}
