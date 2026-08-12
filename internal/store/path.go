package store

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

func NormalizeRootPath(rootPath string) (string, error) {
	if strings.TrimSpace(rootPath) == "" {
		return "", fmt.Errorf("store: root path is required")
	}
	abs, err := filepath.Abs(rootPath)
	if err != nil {
		return "", fmt.Errorf("store: resolving root path %q: %w", rootPath, err)
	}
	normalized := filepath.ToSlash(abs)
	if runtime.GOOS == "windows" {
		normalized = strings.ToLower(normalized)
	}
	return normalized, nil
}
