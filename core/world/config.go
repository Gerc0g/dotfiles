package world

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Config holds the `key: value` pairs of a .company-config or .product-config
// file. Values are read once at scan time and never mutated afterwards.
type Config struct {
	values map[string]string
}

// Get returns the value for key, or an empty string when the key is absent.
func (c Config) Get(key string) string {
	return c.values[key]
}

// Keys returns the configured keys in sorted order.
func (c Config) Keys() []string {
	keys := make([]string, 0, len(c.values))
	for key := range c.values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Len reports how many keys the config carries.
func (c Config) Len() int {
	return len(c.values)
}

// readConfig parses a marker file. A missing file is not an error: the caller
// has already decided the directory qualifies, and an empty config is valid.
func readConfig(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{values: map[string]string{}}, nil
		}
		return Config{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := parseLine(scanner.Text())
		if ok {
			values[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	return Config{values: values}, nil
}

// parseLine splits one `key: value` line. Blank lines and # comments are
// skipped, reported by ok == false.
func parseLine(line string) (key, value string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}

	key, value, found := strings.Cut(trimmed, ":")
	if !found {
		return "", "", false
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return "", "", false
	}

	return key, strings.TrimSpace(value), true
}
