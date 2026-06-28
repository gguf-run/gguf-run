// Package ggufrun provides functions to find, download, and run GGUF models
// with llama.cpp.
package ggufrun

import (
	"context"

	"github.com/gguf-run/finder/finder"
)

// Search queries Hugging Face Hub for GGUF models matching the query.
// Results are sorted by download count (descending).
func Search(ctx context.Context, query string, limit int) ([]finder.Result, error) {
	return finder.Search(ctx, query, limit)
}
