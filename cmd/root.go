package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/daemon"
	"github.com/vladimirvivien/robo/internal/engine"
	"github.com/vladimirvivien/robo/internal/engine/cloud"
	"github.com/vladimirvivien/robo/internal/engine/local"
	"github.com/vladimirvivien/robo/internal/engine/router"
	"github.com/vladimirvivien/robo/internal/shell"
	"github.com/vladimirvivien/robo/internal/ui"
)

var (
	cfgFile            string
	flagLocal          bool
	flagCloud          bool
	flagOutput         string
	flagSystem         string
	flagNoStream       bool
	flagAutoAccept     bool
	flagYoloApproveAll bool
)

// RootCmd represents the base command when called without subcommands.
var RootCmd = &cobra.Command{
	Use:   "robo [intent]",
	Short: "robo: on-device AI terminal companion and shell assistant",
	Long: `robo translates natural-language intents into executable shell commands,
inspects terminal diagnostics, and executes actions with safety guardrails.`,
	Args:          cobra.ArbitraryArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runRoot,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	err := RootCmd.Execute()
	if err != nil {
		if ui.IsStdoutTerminal() && (flagOutput == "markdown" || flagOutput == "md" || flagOutput == "") {
			fmt.Fprintln(os.Stderr, ui.ErrorCard(err.Error()))
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
	}
	return err
}

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	log.SetOutput(io.Discard)

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.robo/config.yaml)")
	RootCmd.PersistentFlags().StringVarP(&flagOutput, "output", "o", "markdown", "output format (markdown, plain, json, code)")
	RootCmd.Flags().BoolVarP(&flagLocal, "local-only", "l", false, "force execution on local on-device SLM")
	RootCmd.Flags().BoolVarP(&flagCloud, "cloud-only", "c", false, "force execution on cloud frontier model")
	RootCmd.Flags().StringVar(&flagSystem, "system", "", "custom system prompt override")
	RootCmd.Flags().BoolVar(&flagNoStream, "no-stream", false, "disable streaming output")
	RootCmd.Flags().BoolVarP(&flagAutoAccept, "auto-accept", "y", false, "auto-accept all non-destructive actions without prompt")
	RootCmd.Flags().BoolVar(&flagYoloApproveAll, "yolo-approve-all", false, "auto-accept and execute all actions including destructive ones (no prompts)")
}

func runRoot(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// 0. Check for config file specification
	targetCfgPath := cfgFile
	if targetCfgPath == "" {
		targetCfgPath = config.ConfigPath()
	}

	if !config.ConfigFileExists(targetCfgPath) {
		if ui.IsStdoutTerminal() && ui.IsStdinTerminal() {
			fmt.Println()
			fmt.Println(ui.Card(
				ui.BadgeWarning("robo • Not Initialized"),
				fmt.Sprintf("Configuration was not found at %s.", targetCfgPath),
				"Setup required",
			))
			fmt.Println()

			confirmed, err := ui.PromptConfirm("Would you like to initialize robo and configure local models now?")
			if err == nil && confirmed {
				if initErr := runInit(cmd, nil); initErr != nil {
					return initErr
				}
				if len(args) == 0 {
					return nil
				}
			} else {
				if cfgFile != "" {
					return fmt.Errorf("config file not found: %s\nSpecify a valid configuration file path, or run 'robo init'", cfgFile)
				}
				return fmt.Errorf("robo is not initialized: config file not found at %s\nRun 'robo init' to set up local models and configuration", targetCfgPath)
			}
		} else {
			if cfgFile != "" {
				return fmt.Errorf("config file not found: %s\nSpecify a valid configuration file path, or run 'robo init'", cfgFile)
			}
			return fmt.Errorf("robo is not initialized: config file not found at %s\nRun 'robo init' to set up local models and configuration", targetCfgPath)
		}
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Apply CLI flags to config overrides
	if flagAutoAccept {
		cfg.Shell.AutoAccept = true
	}
	if flagYoloApproveAll {
		cfg.Shell.YoloApproveAll = true
	}

	// 1. Read prompt and stdin
	var prompt string
	var stdinContent string

	if len(args) > 0 {
		prompt = strings.Join(args, " ")
	} else if !ui.IsStdinTerminal() {
		// Only read stdin when no args provided and stdin is piped
		data, err := io.ReadAll(os.Stdin)
		if err == nil && len(data) > 0 {
			prompt = strings.TrimSpace(string(data))
		}
	}

	if prompt == "" {
		return cmd.Help()
	}

	// 2. Resolve effective output format
	outputFormat := flagOutput
	if !cmd.Flags().Changed("output") && cfg.Shell.OutputMode != "" {
		outputFormat = cfg.Shell.OutputMode
	}
	cfg.Shell.OutputMode = outputFormat

	if _, err := ui.NewFormatter(outputFormat, false, 80); err != nil {
		return err
	}

	// 3. Validate inference environment setup
	forceBackend := ""
	if flagLocal {
		forceBackend = "local-only"
	} else if flagCloud {
		forceBackend = "cloud-only"
	}
	if err := engine.ValidateInferenceSetup(cfg, forceBackend); err != nil {
		// If local inference dependencies are missing in interactive terminal, prompt to re-initialize
		if ui.IsStdoutTerminal() && ui.IsStdinTerminal() && cfg.LLM.Local.Enabled {
			status := engine.CheckLocalSetup(cfg.LLM.Local)
			if !status.HasLib || !status.HasModel {
				fmt.Println()
				fmt.Println(ui.Card(
					ui.BadgeWarning("robo • Setup Required"),
					err.Error(),
					"",
				))
				fmt.Println()
				confirmed, promptErr := ui.PromptConfirm("Would you like to run 'robo init' to download missing dependencies now?")
				if promptErr == nil && confirmed {
					if initErr := runInit(cmd, nil); initErr != nil {
						return initErr
					}
					if len(args) == 0 {
						return nil
					}
					// Reload config after re-init
					if reloaded, loadErr := config.Load(cfgFile); loadErr == nil {
						cfg = reloaded
					}
				} else {
					return err
				}
			} else {
				return err
			}
		} else {
			return err
		}
	}

	// 4. Start visual spinner immediately upon prompt receipt
	isInteractive := ui.IsStdoutTerminal() && (outputFormat == "markdown" || outputFormat == "md" || outputFormat == "")
	var sp *ui.Spinner
	if isInteractive {
		sp = ui.StartSpinner("Working...")
	}
	defer func() {
		if sp != nil {
			sp.Stop()
		}
	}()

	// 5. Assemble ambient shell context (OS, Architecture, active shell, and recent shell history)
	var systemPrompt strings.Builder
	systemPrompt.WriteString(config.DefaultRoboSystemPrompt)
	systemPrompt.WriteString("\n\n")

	// Inject OS / Architecture / Shell environment target
	fmt.Fprintf(&systemPrompt, "[Runtime Target]\nOS: %s\nArchitecture: %s\nActive Shell: %s\n\n", runtime.GOOS, runtime.GOARCH, shell.DetectShell())

	if flagSystem != "" {
		systemPrompt.WriteString("User Instructions:\n")
		systemPrompt.WriteString(flagSystem)
		systemPrompt.WriteString("\n\n")
	}

	if cfg.Shell.CaptureHistory {
		collector := shell.NewCollector(nil)
		maxLines := cfg.Shell.MaxHistoryLines
		if maxLines <= 0 {
			maxLines = 10
		}
		sc, err := collector.Collect(ctx, maxLines)
		if err == nil && sc != nil {
			systemPrompt.WriteString(sc.FormatPromptContext())
			systemPrompt.WriteString("\n")
		}
	}

	// 6. Construct engines
	inProcEngine := local.New(cfg.LLM.Local, cfg)
	localClient := daemon.NewClient(*cfg, daemon.WithInProcEngine(inProcEngine))
	cloudEngine := cloud.New(cfg.LLM.Cloud, cfg)

	r := router.NewRouter(localClient, cloudEngine, cfg.LLM)
	defer func() { _ = r.Close() }()

	// 7. Build request
	req := engine.Request{
		Prompt:       prompt,
		SystemPrompt: systemPrompt.String(),
	}

	if stdinContent != "" {
		req.ContextFiles = append(req.ContextFiles, engine.FileContext{
			Path:    "stdin",
			Content: stdinContent,
		})
	}

	if flagLocal {
		req.ForceBackend = "local-only"
	} else if flagCloud {
		req.ForceBackend = "cloud-only"
	}

	// 8. Execute generation
	var fullText strings.Builder
	var proposedToolCalls []engine.ToolCall

	stream, err := r.GenerateStream(ctx, req)
	if err != nil {
		if sp != nil {
			sp.Stop()
		}
		return err
	}

	for chunk := range stream {
		if chunk.Error != nil {
			if sp != nil {
				sp.Stop()
			}
			fmt.Fprintf(os.Stderr, "\n[stream error]: %v\n", chunk.Error)
			return chunk.Error
		}
		fullText.WriteString(chunk.Text)
		if len(chunk.ToolCalls) > 0 {
			proposedToolCalls = append(proposedToolCalls, chunk.ToolCalls...)
		}
	}
	if sp != nil {
		sp.Stop()
	}

	rawResponse := fullText.String()
	cleaned := ui.CleanResponseText(rawResponse)
	cmdStr := ""
	explanation := strings.TrimSpace(shell.StripCodeBlock(cleaned))

	// Structured tool calls take precedence over heuristic markdown code blocks
	if len(proposedToolCalls) > 0 {
		cmdStr = proposedToolCalls[0].Command
		if proposedToolCalls[0].Description != "" && (explanation == "" || explanation == cleaned) {
			explanation = proposedToolCalls[0].Description
		}
	} else {
		cmdStr = shell.ExtractProposedCommand(cleaned)
	}

	usedLocal := flagLocal || (!flagCloud && cfg.LLM.Local.Enabled)
	providerName := cfg.LLM.Local.Provider
	modelName := cfg.LLM.Local.Model
	if !usedLocal {
		providerName = cfg.LLM.Cloud.Provider
		modelName = cfg.LLM.Cloud.Model
	}

	formatter, err := ui.NewFormatter(outputFormat, isInteractive, ui.TerminalWidth())
	if err != nil {
		return err
	}

	outputData := ui.OutputData{
		Response:    cleaned,
		Explanation: explanation,
		Command:     cmdStr,
		Output:      cleaned,
		Provider:    providerName,
		Model:       modelName,
		Local:       usedLocal,
	}

	// Render output
	if err := formatter.Format(os.Stdout, outputData); err != nil {
		return err
	}

	// In interactive mode, if a command was proposed, orchestrator executes interactive review
	if isInteractive && cmdStr != "" {
		toolHandler := shell.NewToolHandler(cfg)
		_, _ = toolHandler.Handle(ctx, shell.ShellInput{
			Command:     cmdStr,
			Description: explanation,
		})
	}

	return nil
}
