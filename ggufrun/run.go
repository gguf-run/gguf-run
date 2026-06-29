package ggufrun

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gguf-run/gguf-run/ggufrun/llama"
)

// Run loads a model and runs interactive generation using the embedded llama.cpp library.
func Run(modelPath string, opts *RunOptions) error {
	if err := llama.Init(); err != nil {
		return fmt.Errorf("llama init: %w", err)
	}
	defer llama.Close()

	if llama.LlamaBackendInit != nil {
		llama.LlamaBackendInit()
	}
	defer func() {
		if llama.LlamaBackendFree != nil {
			llama.LlamaBackendFree()
		}
	}()

	model, err := llama.LoadModel(modelPath)
	if err != nil {
		return fmt.Errorf("load model: %w", err)
	}
	defer model.Free()

	ctxSize := opts.ContextSize
	if ctxSize <= 0 {
		ctxSize = 4096
	}
	ctx, err := model.NewContext(ctxSize)
	if err != nil {
		return fmt.Errorf("create context: %w", err)
	}
	defer ctx.Free()

	if opts.Prompt == "" {
		return runInteractive(ctx, opts)
	}

	return runSingleShot(ctx, opts)
}

// RunOptions contains options for Run().
type RunOptions struct {
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
	ContextSize     int32
	Threads         int32
	GPULayers       int32
	UseMMap         bool
	UseMLock        bool
}

// RunServer starts an HTTP server with the model loaded via embedded llama.cpp.
func RunServer(modelPath, addr string, opts *ServerOptions) error {
	if err := llama.Init(); err != nil {
		return fmt.Errorf("llama init: %w", err)
	}
	defer llama.Close()

	if llama.LlamaBackendInit != nil {
		llama.LlamaBackendInit()
	}
	defer func() {
		if llama.LlamaBackendFree != nil {
			llama.LlamaBackendFree()
		}
	}()

	model, err := llama.LoadModel(modelPath)
	if err != nil {
		return fmt.Errorf("load model: %w", err)
	}
	defer model.Free()

	srv := &Server{
		model: model,
		opts:  opts,
	}

	fmt.Fprintf(os.Stderr, "\n\033[32m==>\033[0m Server listening on %s\n", addr)
	return http.ListenAndServe(addr, srv)
}

// ServerOptions contains options for RunServer().
type ServerOptions struct {
	ContextSize int32
	Threads     int32
	GPULayers   int32
	UseMMap     bool
	UseMLock    bool
	APIKeys     []string
}

// Server implements http.Handler for the OpenAI-compatible API.
type Server struct {
	model *llama.Model
	opts  *ServerOptions
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(s.opts.APIKeys) > 0 {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		valid := false
		for _, key := range s.opts.APIKeys {
			if auth == "Bearer "+key {
				valid = true
				break
			}
		}
		if !valid {
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}
	}

	switch r.URL.Path {
	case "/v1/completions", "/v1/chat/completions":
		s.handleCompletions(w, r)
	case "/health":
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error": "inference not yet implemented"}`))
}

func runInteractive(ctx *llama.Context, opts *RunOptions) error {
	fmt.Fprintln(os.Stderr, "\n\033[32m==>\033[0m Interactive mode (Ctrl+C to exit)")
	fmt.Fprintln(os.Stderr, "Type your prompt and press Enter. Empty line to submit.")

	for {
		fmt.Fprint(os.Stderr, "\n> ")
		var input string
		_, err := fmt.Scanln(&input)
		if err != nil {
			if err.Error() == "unexpected newline" {
				continue
			}
			break
		}

		if input == "" {
			continue
		}

		if strings.EqualFold(input, "/exit") || strings.EqualFold(input, "/quit") {
			break
		}

		genOpts := llama.GenerateOptions{
			Prompt:          input,
			MaxTokens:       opts.MaxTokens,
			Temperature:     opts.Temperature,
			TopK:            opts.TopK,
			TopP:            opts.TopP,
			MinP:            opts.MinP,
			TypicalP:        opts.TypicalP,
			RepeatPenalty:   opts.RepeatPenalty,
			FreqPenalty:     opts.FreqPenalty,
			PresencePenalty: opts.PresencePenalty,
			Seed:            opts.Seed,
			Stream:          true,
			Callback:        func(token string) { fmt.Fprint(os.Stdout, token) },
		}

		if _, err := ctx.Generate(genOpts); err != nil {
			fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		}
		fmt.Fprintln(os.Stdout)
	}

	return nil
}

func runSingleShot(ctx *llama.Context, opts *RunOptions) error {
	genOpts := llama.GenerateOptions{
		Prompt:           opts.Prompt,
		MaxTokens:        opts.MaxTokens,
		Temperature:      opts.Temperature,
		TopK:             opts.TopK,
		TopP:             opts.TopP,
		MinP:             opts.MinP,
		TypicalP:         opts.TypicalP,
		RepeatPenalty:    opts.RepeatPenalty,
		FreqPenalty:      opts.FreqPenalty,
		PresencePenalty:  opts.PresencePenalty,
		Seed:             opts.Seed,
		Stream:           true,
		Callback:         func(token string) { fmt.Fprint(os.Stdout, token) },
	}

	if _, err := ctx.Generate(genOpts); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout)
	return nil
}
