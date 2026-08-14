package secret

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Cache stores 1Password field values on disk with a TTL, so direnv does not
// trigger Touch ID on every cd. The key scheme (sha256 of "vault|item|field")
// matches the bash predecessor, so existing cache entries stay valid.
type Cache struct {
	Root string
	TTL  time.Duration
	// Bypass forces a re-read even when the entry is fresh.
	Bypass bool
	// read fetches the value from 1Password; a field so tests can stub it.
	read func(vault, item, field string) (string, error)
}

// NewCache resolves the cache location and TTL from the environment:
// SECRET_CACHE_DIR, SECRET_CACHE_TTL_DAYS, SECRET_CACHE_BYPASS.
func NewCache() (*Cache, error) {
	root := os.Getenv("SECRET_CACHE_DIR")
	if root == "" {
		base := os.Getenv("XDG_CACHE_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("resolve home directory: %w", err)
			}
			base = filepath.Join(home, ".cache")
		}
		root = filepath.Join(base, "dotfiles", "secrets")
	}

	ttlDays := 30
	if v := os.Getenv("SECRET_CACHE_TTL_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			ttlDays = n
		}
	}

	return &Cache{
		Root:   root,
		TTL:    time.Duration(ttlDays) * 24 * time.Hour,
		Bypass: os.Getenv("SECRET_CACHE_BYPASS") == "1",
		read:   opReadField,
	}, nil
}

func cacheKey(vault, item, field string) string {
	sum := sha256.Sum256([]byte(vault + "|" + item + "|" + field))
	return hex.EncodeToString(sum[:])
}

func (c *Cache) valueFile(key string) string { return filepath.Join(c.Root, key+".value") }
func (c *Cache) metaFile(key string) string  { return filepath.Join(c.Root, key+".meta") }

func (c *Cache) ensureDir() error {
	if err := os.MkdirAll(c.Root, 0o700); err != nil {
		return fmt.Errorf("создание %s: %w", c.Root, err)
	}
	return os.Chmod(c.Root, 0o700)
}

// Get returns the cached value, refreshing from 1Password when stale. A zero
// ttl uses the cache default.
func (c *Cache) Get(vault, item, field string, ttl time.Duration) (string, error) {
	if field == "" {
		field = "credential"
	}
	if ttl == 0 {
		ttl = c.TTL
	}
	if err := c.ensureDir(); err != nil {
		return "", err
	}

	key := cacheKey(vault, item, field)
	file := c.valueFile(key)

	if !c.Bypass {
		if info, err := os.Stat(file); err == nil && time.Since(info.ModTime()) < ttl {
			data, err := os.ReadFile(file)
			if err == nil {
				return string(data), nil
			}
		}
	}

	value, err := c.read(vault, item, field)
	if err != nil {
		return "", err
	}
	if err := c.write(key, vault, item, field, value); err != nil {
		return "", err
	}
	return value, nil
}

func (c *Cache) write(key, vault, item, field, value string) error {
	file := c.valueFile(key)
	tmp, err := os.CreateTemp(c.Root, filepath.Base(file)+".*")
	if err != nil {
		return fmt.Errorf("временный файл в %s: %w", c.Root, err)
	}
	defer os.Remove(tmp.Name())

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(value); err != nil {
		tmp.Close()
		return fmt.Errorf("запись кэша: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), file); err != nil {
		return fmt.Errorf("замена %s: %w", file, err)
	}

	meta := fmt.Sprintf("vault=%s\nitem=%s\nfield=%s\nupdated_at=%s\n",
		vault, item, field, time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err := os.WriteFile(c.metaFile(key), []byte(meta), 0o600); err != nil {
		return fmt.Errorf("запись %s: %w", c.metaFile(key), err)
	}
	return nil
}

// Clear removes one entry, or every entry when vault is empty.
func (c *Cache) Clear(vault, item, field string) error {
	if err := c.ensureDir(); err != nil {
		return err
	}
	if vault == "" {
		for _, pattern := range []string{"*.value", "*.meta"} {
			matches, _ := filepath.Glob(filepath.Join(c.Root, pattern))
			for _, match := range matches {
				_ = os.Remove(match)
			}
		}
		return nil
	}
	if field == "" {
		field = "credential"
	}
	key := cacheKey(vault, item, field)
	_ = os.Remove(c.valueFile(key))
	_ = os.Remove(c.metaFile(key))
	return nil
}

// List renders one line per cached entry from the meta files.
func (c *Cache) List() ([]string, error) {
	if err := c.ensureDir(); err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(filepath.Join(c.Root, "*.meta"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)

	var out []string
	for _, meta := range matches {
		data, err := os.ReadFile(meta)
		if err != nil {
			continue
		}
		var parts []string
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if key, value, found := strings.Cut(line, "="); found {
				parts = append(parts, key+": "+value)
			}
		}
		out = append(out, strings.Join(parts, " | "))
	}
	return out, nil
}
