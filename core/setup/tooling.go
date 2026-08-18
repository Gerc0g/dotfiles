package setup

import (
	"fmt"
	"os/exec"
	"strings"
)

// The platform shells out to a handful of binaries. When one is missing the
// failure surfaces far from the cause — a routine that dies on "codex: not
// found", a company created without its 1Password vault — so the check belongs
// where the rest of the machine state is checked.
//
// Installing them is bootstrap's job: it needs the network and a package
// manager. This step only reports.

type tool struct {
	name string
	// why says what stops working, so a missing binary is actionable rather
	// than a name on a list.
	why string
	// required marks a binary the platform cannot work without at all.
	required bool
}

var tools = []tool{
	{"git", "модель мира, коммиты, воркспейсы", true},
	{"go", "сборка ядра (make build)", true},
	{"python3", "скраббер транскриптов", true},
	{"codex", "куратор памяти и агент зон вольта", true},
	{"tmux", "сессии агентов", false},
	{"direnv", "переменные окружения проектов", false},
	{"op", "1Password: vault компаний", false},
	{"gh", "hq finish: draft PR", false},
	{"claude", "routine с agent: claude и headless-прогоны", false},
}

func toolingStep() Step {
	return Step{
		Name:  "tooling",
		About: "внешние CLI, на которые платформа опирается",
		check: func(env Env) Result {
			var missingRequired, missingOptional []string
			for _, t := range tools {
				if _, err := exec.LookPath(t.name); err == nil {
					continue
				}
				entry := fmt.Sprintf("%s (%s)", t.name, t.why)
				if t.required {
					missingRequired = append(missingRequired, entry)
				} else {
					missingOptional = append(missingOptional, entry)
				}
			}

			if len(missingRequired) > 0 {
				return drifted("нет обязательных: %s", strings.Join(missingRequired, ", "))
			}
			if len(missingOptional) > 0 {
				// Not drift: the platform works, one capability does not. Saying
				// so beats a red line nobody can act on.
				return ok("нет необязательных: %s", strings.Join(missingOptional, ", "))
			}
			return ok("%d CLI на месте", len(tools))
		},
	}
}
