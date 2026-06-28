package ggufrun

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gguf-run/finder/finder"
)

func Search(ctx context.Context, query string, limit int) ([]finder.Result, error) {
	return finder.Search(ctx, query, limit)
}

type SearchBestResult struct {
	URL      string
	ModelID  string
	Filename string
}

func SearchBest(ctx context.Context, query string, limit int) (*SearchBestResult, error) {
	results, err := finder.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no models found for: %s", query)
	}

	for _, r := range results {
		for _, f := range r.Files {
			if strings.Contains(strings.ToLower(f), "q4_k_m") {
				return &SearchBestResult{
					URL:      f,
					ModelID:  r.ModelID,
					Filename: filepath.Base(f),
				}, nil
			}
		}
	}

	return &SearchBestResult{
		URL:      results[0].Files[0],
		ModelID:  results[0].ModelID,
		Filename: filepath.Base(results[0].Files[0]),
	}, nil
}
