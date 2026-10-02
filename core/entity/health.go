package entity

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Gerc0g/dotfiles/core/workspace"
)

func validGit(path string) bool {
	b, err := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return false
	}
	actual, err := filepath.EvalSymlinks(strings.TrimSpace(string(b)))
	if err != nil {
		return false
	}
	want, err := filepath.EvalSymlinks(path)
	return err == nil && actual == want
}

func health(root string, s snapshot, p Onboarding, parents []Parent) Health {
	h := Health{CheckedAt: now(), Status: "ok", Checks: []Check{}}
	add := func(id, title, status, detail, path string, blocking bool) {
		h.Checks = append(h.Checks, Check{ID: id, Title: title, Status: status, Detail: detail, Path: path, Blocking: blocking})
		if status == "error" {
			h.Status = "error"
		} else if status == "warning" && h.Status != "error" {
			h.Status = "warning"
		}
	}
	add("registration", "Регистрация в HQ", "ok", "Структура и маркеры сущности найдены.", s.path, false)
	if !s.docExists || strings.TrimSpace(string(s.content)) == "" {
		add("context", "Контекст", "warning", "AGENTS.md отсутствует или пуст. Агент может работать с неполным контекстом.", filepath.Join(s.path, "AGENTS.md"), false)
	} else {
		add("context", "Контекст", "ok", "AGENTS.md доступен для чтения.", filepath.Join(s.path, "AGENTS.md"), false)
	}
	if p.Status != "complete" {
		add("onboarding", "Онбординг", "warning", "Есть незаполненные или неподтверждённые разделы. Это не блокирует запуск агента.", filepath.Join(s.path, "AGENTS.md"), false)
	} else {
		add("onboarding", "Онбординг", "ok", "Обязательные разделы заполнены и подтверждены.", filepath.Join(s.path, "AGENTS.md"), false)
	}
	for _, parent := range parents {
		b, err := os.ReadFile(parent.DocumentPath)
		if err != nil || strings.TrimSpace(string(b)) == "" {
			add("parent-"+parent.Kind, "Контекст родителя", "warning", "Контекст "+parent.Scope+" недоступен или пуст.", parent.DocumentPath, false)
		} else {
			add("parent-"+parent.Kind, "Контекст родителя", "ok", parent.Scope+": AGENTS.md доступен.", parent.DocumentPath, false)
		}
	}
	if s.kind == "repo" {
		if validGit(s.path) {
			add("git", "Git", "ok", "Checkout доступен Git.", s.path, false)
		} else {
			add("git", "Git", "error", "Маркер .git существует, но checkout недоступен Git. Создание worktree требует исправления.", s.path, true)
		}
		hot := filepath.Join(s.path, "docs", "knowledge", "hot.md")
		if b, err := os.ReadFile(hot); err != nil || strings.TrimSpace(string(b)) == "" {
			add("memory", "Память", "warning", "docs/knowledge/hot.md недоступен или пуст. WikiPedik может быть ещё не подключён.", hot, false)
		} else {
			add("memory", "Память", "ok", "Файл оперативного контекста WikiPedik доступен; актуальность содержания требует проверки.", hot, false)
		}
		manager, err := workspace.New(root, workspace.WithProjectSync(func() {}))
		if err == nil {
			var ws []workspace.Workspace
			ws, err = manager.List()
			if err == nil {
				for _, w := range ws {
					if w.Company != s.parts[0] || w.Product != s.parts[1] || w.Repo != s.parts[2] {
						continue
					}
					issues, e := manager.ContextIssues(w)
					for i := range issues {
						issue := issues[i]
						add("worktree-"+w.ID+"-"+strings.TrimPrefix(issue.Path, w.Path), "Контекст worktree "+w.ID, issue.Severity, issue.Message, issue.Path, false)
					}
					if e != nil && len(issues) == 0 {
						add("worktree-"+w.ID, "Контекст worktree", "error", e.Error(), w.Path, false)
					}
				}
			}
		}
		if err != nil {
			add("worktrees", "Worktree", "unknown", err.Error(), s.path, false)
		}
	}
	add("runtime", "Проверки приложения", "unknown", "Сборка, тесты, сеть и работоспособность приложения этой проверкой не запускаются.", "", false)
	return h
}
