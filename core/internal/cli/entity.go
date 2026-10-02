package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Gerc0g/dotfiles/core/entity"
	"github.com/Gerc0g/dotfiles/core/world"
	"github.com/spf13/cobra"
)

func newEntityCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "entity", Short: "Карточки контекста и онбординга HQ"}
	var asJSON bool
	show := &cobra.Command{Use: "show <company[/product[/repo]]>", Short: "Контекст, прогресс и локальные проверки", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		root, err := world.Root()
		if err != nil {
			return err
		}
		card, err := entity.Show(root, args[0])
		if err != nil {
			return err
		}
		if asJSON {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(card)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\nContext: %s\nOnboarding: %s (%d/%d)\nHealth: %s\n", card.Scope, card.Kind, card.Document.Path, card.Onboarding.Status, card.Onboarding.Completed, card.Onboarding.Total, card.Health.Status)
		for _, item := range card.Onboarding.Items {
			fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s: %s\n", item.Status, item.ID, item.Title)
		}
		for _, check := range card.Health.Checks {
			if check.Status != "ok" {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s: %s\n", check.Status, check.Title, check.Detail)
			}
		}
		return nil
	}}
	show.Flags().BoolVar(&asJSON, "json", false, "JSON для интерфейсов и агентов")
	var encoded string
	update := &cobra.Command{Use: "update <company[/product[/repo]]>", Short: "Сохранить одно изменение с проверкой версии", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		body, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return fmt.Errorf("invalid base64 payload: %w", err)
		}
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		var request entity.UpdateRequest
		if err = decoder.Decode(&request); err != nil {
			return fmt.Errorf("invalid update payload: %w", err)
		}
		if err = decoder.Decode(new(any)); err != io.EOF {
			return fmt.Errorf("update payload must contain one JSON object")
		}
		root, err := world.Root()
		if err != nil {
			return err
		}
		card, err := entity.Update(root, args[0], request)
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(card)
	}}
	update.Flags().StringVar(&encoded, "json-base64", "", "UTF-8 JSON, закодированный в base64")
	_ = update.MarkFlagRequired("json-base64")
	var action, itemID string
	var taskJSON bool
	task := &cobra.Command{Use: "task <company[/product[/repo]]>", Short: "Подготовить задачу онбординга для отдельного чата", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		root, err := world.Root()
		if err != nil {
			return err
		}
		spec, err := entity.Task(root, args[0], action, itemID)
		if err != nil {
			return err
		}
		if taskJSON {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(spec)
		}
		fmt.Fprintln(cmd.OutOrStdout(), spec.Prompt)
		return nil
	}}
	task.Flags().StringVar(&action, "action", "onboard", "Действие из agentActions карточки")
	task.Flags().StringVar(&itemID, "item", "", "Один ручной пункт онбординга (необязательно)")
	task.Flags().BoolVar(&taskJSON, "json", false, "JSON задачи для интерфейса")
	cmd.AddCommand(show, update, task)
	return cmd
}
