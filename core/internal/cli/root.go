// Package cli wires the hq command surface. It holds no domain logic: every
// command resolves the workspace through the world package, so the CLI, the
// TUIs and any later daemon cannot drift apart the way the zsh functions did.
package cli

import (
	"github.com/spf13/cobra"
)

// Version is stamped at build time with -ldflags "-X ...cli.Version=...".
var Version = "dev"

// NewRoot builds the hq command tree.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "hq",
		Short: "Ядро личного рабочего окружения",
		Long: "hq владеет моделью мира «компания → продукт → репозиторий» и операциями\n" +
			"над ней. Оболочка, TUI, агенты и планировщик — это интерфейсы поверх\n" +
			"ядра, а не вторая копия логики.\n\n" +
			"Вывод подстраивается под получателя: в терминале — цвет и дерево,\n" +
			"в конвейере — простой текст, который разберёт скрипт или агент.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Cobra generates a help flag, a help command and a completion command, all
	// with English text. Defining the flag first wins the race with cobra's
	// default, the help command is replaced outright, and completion stays
	// available but drops out of the command list where it only adds noise.
	root.PersistentFlags().BoolP("help", "h", false, "справка по команде")
	root.SetHelpCommand(&cobra.Command{
		Use:    "help [команда]",
		Short:  "Справка по команде",
		Hidden: true,
	})
	root.CompletionOptions.HiddenDefaultCmd = true

	root.AddCommand(
		newLsCmd(),
		newCtxCmd(),
		newSetupCmd(),
		newDoctorCmd(),
		newServerCmd(),
		newWorkspaceCmd(),
		newEditorCmd(),
		newCommitCmd(),
		newFinishCmd(),
		newSkillCmd(),
		newSecretCmd(),
		newDevstackCmd(),
		newWikiCmd(),
		newWikipedikCmd(),
		newHookCmd(),
		newCommandsCmd(),
		newOnboardCmd(),
	)

	groupStubs(root)
	return root
}

// stubGroupID collects the commands that only hold surface. Fang renders
// cobra groups as separate titled sections, so stubs drop out of the main
// list without any styling hacks — Short-embedded ANSI breaks fang's layout.
const stubGroupID = "stubs"

func groupStubs(root *cobra.Command) {
	root.AddGroup(&cobra.Group{ID: stubGroupID, Title: "заглушки"})
	for _, cmd := range root.Commands() {
		switch cmd.Name() {
		case "secret", "server":
			cmd.GroupID = stubGroupID
		}
	}
}
