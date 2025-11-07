package util

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func LoadFromFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt":
		// Split and clean entries
		raw := strings.Split(string(data), ",")
		entry := make([]string, 0, len(raw))
		for _, u := range raw {
			u = strings.TrimSpace(u) // removes spaces, tabs, and newlines
			if u != "" {
				entry = append(entry, u)
			}
		}
		return entry, nil
	default:
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}
}
