package ggufrun

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CachedModel struct {
	Name    string
	Size    int64
	ModTime time.Time
	Path    string
}

func ListCached(cacheDir string) ([]CachedModel, error) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var models []CachedModel
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".gguf") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		models = append(models, CachedModel{
			Name:    e.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Path:    filepath.Join(cacheDir, e.Name()),
		})
	}
	return models, nil
}

func RemoveCached(name, cacheDir string) error {
	path := filepath.Join(cacheDir, name)
	if _, err := os.Stat(path); err == nil {
		return os.Remove(path)
	}

	if !strings.HasSuffix(name, ".gguf") {
		path = filepath.Join(cacheDir, name+".gguf")
		if _, err := os.Stat(path); err == nil {
			return os.Remove(path)
		}
	}

	models, err := ListCached(cacheDir)
	if err != nil {
		return fmt.Errorf("cannot list cache: %w", err)
	}

	lower := strings.ToLower(name)
	var matched []string
	for _, m := range models {
		if strings.Contains(strings.ToLower(m.Name), lower) {
			matched = append(matched, m.Path)
		}
	}

	switch len(matched) {
	case 0:
		return fmt.Errorf("no model matching %q found in cache", name)
	case 1:
		return os.Remove(matched[0])
	default:
		return fmt.Errorf("multiple models match %q:\n  %s", name, strings.Join(matched, "\n  "))
	}
}
