package util

import (
	"bufio"
	"os"
	"strings"
)

func LoadFromYamlFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var items []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse YAML array items (lines starting with "- ")
		if strings.HasPrefix(line, "- ") {
			value := strings.TrimSpace(line[2:])
			// Remove quotes if present
			value = strings.Trim(value, "\"'")
			items = append(items, value)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return items, nil
}
