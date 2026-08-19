package ui

import (
	"fmt"
	"strings"

	"charm.land/huh/v2"
)

// Action represents the user's choice for proposed command execution.
type Action string

const (
	ActionRun    Action = "run"
	ActionEdit   Action = "edit"
	ActionCancel Action = "cancel"
)

// PromptCommandReview presents [Run] [Edit] [Cancel] for a proposed command.
func PromptCommandReview(command string) (Action, string, error) {
	var selected string

	selectField := huh.NewSelect[string]().
		Title("Execute command?").
		Options(
			huh.NewOption("Run command", string(ActionRun)),
			huh.NewOption("Edit command", string(ActionEdit)),
			huh.NewOption("Cancel", string(ActionCancel)),
		).
		Value(&selected)

	form := huh.NewForm(huh.NewGroup(selectField))
	if err := form.Run(); err != nil {
		return ActionCancel, command, err
	}

	action := Action(selected)
	if action == ActionEdit {
		edited := command
		inputField := huh.NewInput().
			Title("Edit command:").
			Value(&edited)

		editForm := huh.NewForm(huh.NewGroup(inputField))
		if err := editForm.Run(); err != nil {
			return ActionCancel, command, err
		}
		return ActionRun, strings.TrimSpace(edited), nil
	}

	return action, command, nil
}

// PromptDestructiveConfirm prompts the user to type confirmation for dangerous commands.
func PromptDestructiveConfirm(warning, requiredKeyword string) (bool, error) {
	if requiredKeyword == "" {
		requiredKeyword = "yes-delete"
	}

	var typedInput string
	inputField := huh.NewInput().
		Title(fmt.Sprintf("%s\nType %q to confirm execution:", warning, requiredKeyword)).
		Value(&typedInput)

	form := huh.NewForm(huh.NewGroup(inputField))
	if err := form.Run(); err != nil {
		return false, err
	}

	return strings.TrimSpace(typedInput) == requiredKeyword, nil
}

// PromptConfirm prompts for simple boolean confirmation.
func PromptConfirm(title string) (bool, error) {
	var confirmed bool
	confirmField := huh.NewConfirm().
		Title(title).
		Value(&confirmed)

	form := huh.NewForm(huh.NewGroup(confirmField))
	if err := form.Run(); err != nil {
		return false, err
	}

	return confirmed, nil
}

// PromptModelSelection prompts the user to select an on-device model during initial setup.
func PromptModelSelection() (string, error) {
	var selected string

	selectField := huh.NewSelect[string]().
		Title("Select an on-device model to download:").
		Description("Robo runs locally on your machine using Google LiteRT-LM. Choose a model size:").
		Options(
			huh.NewOption("Gemma 4 4B (Recommended — High Capability ~3.7 GB)", "litert-community/gemma-4-E4B-it-litert-lm"),
			huh.NewOption("Gemma 4 2B (Fast & Lightweight ~2.6 GB)", "litert-community/gemma-4-E2B-it-litert-lm"),
		).
		Value(&selected)

	form := huh.NewForm(huh.NewGroup(selectField))
	if err := form.Run(); err != nil {
		return "", err
	}

	return selected, nil
}
