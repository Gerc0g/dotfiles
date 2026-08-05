package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// checkDir reports whether dir exists as a directory.
func checkDir(dir string) Result {
	info, err := os.Stat(dir)
	switch {
	case os.IsNotExist(err):
		return missing("%s", short(dir))
	case err != nil:
		return drifted("%s: %v", short(dir), err)
	case !info.IsDir():
		return drifted("%s существует, но это не каталог", short(dir))
	default:
		return ok("%s", short(dir))
	}
}

func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	return nil
}

// checkSymlink reports whether dst is a symlink pointing at src.
func checkSymlink(src, dst string) Result {
	info, err := os.Lstat(dst)
	if os.IsNotExist(err) {
		return missing("%s → %s", short(dst), short(src))
	}
	if err != nil {
		return drifted("%s: %v", short(dst), err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return drifted("%s — обычный файл, а не ссылка на %s", short(dst), short(src))
	}

	target, err := os.Readlink(dst)
	if err != nil {
		return drifted("%s: ссылку не прочитать: %v", short(dst), err)
	}
	if target != src {
		return drifted("%s → %s (ожидается %s)", short(dst), short(target), short(src))
	}

	if _, err := os.Stat(dst); err != nil {
		return drifted("%s → %s ведёт в никуда", short(dst), short(src))
	}

	return ok("%s → %s", short(dst), short(src))
}

// ensureSymlink points dst at src. A pre-existing regular file is copied aside
// first: onboarding must never silently destroy something the user wrote.
func ensureSymlink(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("source %s is not readable: %w", src, err)
	}

	if err := ensureDir(filepath.Dir(dst)); err != nil {
		return err
	}

	if info, err := os.Lstat(dst); err == nil && info.Mode()&os.ModeSymlink == 0 {
		backup := fmt.Sprintf("%s.bak.%s", dst, time.Now().Format("20060102150405"))
		if err := os.Rename(dst, backup); err != nil {
			return fmt.Errorf("back up %s: %w", dst, err)
		}
	}

	if err := os.RemoveAll(dst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("replace %s: %w", dst, err)
	}
	if err := os.Symlink(src, dst); err != nil {
		return fmt.Errorf("link %s → %s: %w", dst, src, err)
	}

	return nil
}

// checkLineInFile reports whether path contains a line matching needle.
func checkLineInFile(path, needle string) Result {
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return missing("в %s нет строки загрузчика", short(path))
	}
	if err != nil {
		return drifted("%s: %v", short(path), err)
	}

	if strings.Contains(string(body), needle) {
		return ok("%s подключает загрузчик", short(path))
	}
	return missing("в %s нет строки загрузчика", short(path))
}

// appendBlock adds block to the end of path, creating the file if needed.
func appendBlock(path, block string) error {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	if _, err := file.WriteString(block); err != nil {
		return fmt.Errorf("append to %s: %w", path, err)
	}

	return nil
}

// short replaces the home prefix with ~ so output stays narrow.
func short(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + path[len(home):]
	}
	return path
}
