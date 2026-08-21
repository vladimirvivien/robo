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
	StopActiveSpinner()

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
	StopActiveSpinner()

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
	StopActiveSpinner()

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

// ModelChoice represents a model option displayed in the initialization wizard.
type ModelChoice struct {
	ID          string
	Description string
	Default     bool
}

// InitPreferences holds choices selected in the robo init wizard.
type InitPreferences struct {
	Version string
	Model   string
	Backend string
}

// PromptInitSelection prompts the user to select a LiteRT-LM runtime version, on-device model, and acceleration backend.
func PromptInitSelection(modelChoices ...ModelChoice) (InitPreferences, error) {
	StopActiveSpinner()

	defaultModel := "litert-community/gemma-4-E4B-it"
	for _, m := range modelChoices {
		if m.Default {
			defaultModel = m.ID
			break
		}
	}

	prefs := InitPreferences{
		Version: "v0.16.0",
		Model:   defaultModel,
		Backend: "gpu",
	}

	versionSelect := huh.NewSelect[string]().
		Title("Select LiteRT-LM runtime library version:").
		Options(
			huh.NewOption("v0.16.0 • Recommended (Default)", "v0.16.0"),
		).
		Value(&prefs.Version)

	var modelOptions []huh.Option[string]
	if len(modelChoices) > 0 {
		for _, m := range modelChoices {
			modelOptions = append(modelOptions, huh.NewOption(m.Description, m.ID))
		}
	} else {
		modelOptions = []huh.Option[string]{
			huh.NewOption("Gemma 4 4B  [~2.6 GB] • Recommended (Balanced reasoning & speed)", "litert-community/gemma-4-E4B-it"),
			huh.NewOption("Gemma 4 2B  [~1.6 GB] • Fast & lightweight on-device", "litert-community/gemma-4-E2B-it"),
			huh.NewOption("Gemma 4 12B [~7.5 GB] • High capability reasoning & coding", "litert-community/gemma-4-12B-it"),
			huh.NewOption("Gemma 3 1B  [~580 MB] • Ultra-compact on-device SLM", "litert-community/gemma3-1b-it-int4"),
			huh.NewOption("Qwen 3 4B   [~2.5 GB] • Multilingual & strong tool calling", "litert-community/qwen3-4b-it"),
		}
	}

	modelSelect := huh.NewSelect[string]().
		Title("Select an on-device language model:").
		Description("Models run privately on your local hardware via Google LiteRT-LM:").
		Options(modelOptions...).
		Value(&prefs.Model)

	backendSelect := huh.NewSelect[string]().
		Title("Select hardware acceleration backend:").
		Options(
			huh.NewOption("GPU (Direct3D 12 / Metal / Vulkan)", "gpu"),
			huh.NewOption("CPU (Multi-threaded compute)", "cpu"),
		).
		Value(&prefs.Backend)

	form := huh.NewForm(huh.NewGroup(versionSelect, modelSelect, backendSelect))
	if err := form.Run(); err != nil {
		return prefs, err
	}

	return prefs, nil
}

// PromptModelSelection prompts the user to select an on-device model.
func PromptModelSelection() (string, error) {
	prefs, err := PromptInitSelection()
	if err != nil {
		return "", err
	}
	return prefs.Model, nil
}
