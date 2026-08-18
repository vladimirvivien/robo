package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
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
	Use:   "robo [prompt]",
	Short: "robo: AI-native developer companion and terminal assistant",
	Long: `robo is a two-tier AI assistant with sub-50ms hot-start on-device execution
powered by LiteRT-LM (robod) and intelligent automatic escalation to frontier cloud models.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runRoot,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return RootCmd.Execute()
}

func init() {
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.config/robo/config.yaml)")
	RootCmd.Flags().BoolVarP(&flagLocal, "local-only", "l", false, "force execution on local on-device SLM")
	RootCmd.Flags().BoolVarP(&flagCloud, "cloud-only", "c", false, "force execution on cloud frontier model")
	RootCmd.Flags().StringVarP(&flagOutput, "output", "o", "markdown", "output format (markdown, plain, json, code)")
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

	// 1. Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 2. Read prompt and stdin
	var prompt string
	var stdinContent string

	if !ui.IsStdinTerminal() {
		data, err := io.ReadAll(os.Stdin)
		if err == nil && len(data) > 0 {
			stdinContent = string(data)
		}
	}

	if len(args) > 0 {
		prompt = strings.Join(args, " ")
	} else if stdinContent != "" {
		prompt = stdinContent
		stdinContent = ""
	}

	if prompt == "" {
		return cmd.Help()
	}

	// 3. Assemble ambient shell context
	var systemPrompt strings.Builder
	if flagSystem != "" {
		systemPrompt.WriteString(flagSystem)
		systemPrompt.WriteString("\n\n")
	}

	if cfg.Shell.CaptureHistory {
		collector := shell.NewCollector(nil)
		maxLines := cfg.Shell.MaxHistoryLines
		if maxLines <= 0 {
			maxLines = 5
		}
		sc, err := collector.Collect(ctx, maxLines)
		if err == nil && sc != nil {
			systemPrompt.WriteString(sc.FormatPromptContext())
			systemPrompt.WriteString("\n")
		}
	}

	// 4. Construct engines
	inProcEngine := local.New(cfg.Local)
	localClient := daemon.NewClient(*cfg, daemon.WithInProcEngine(inProcEngine))
	cloudEngine := cloud.New(cfg.Cloud)

	r := router.NewRouter(localClient, cloudEngine, cfg.Routing)
	defer func() { _ = r.Close() }()

	// 5. Build request
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

	// 6. Execute generation
	isInteractive := ui.IsStdoutTerminal() && flagOutput == "markdown"
	var fullText strings.Builder

	if flagNoStream || !isInteractive {
		resp, err := r.Generate(ctx, req)
		if err != nil {
			if isInteractive {
				fmt.Fprintln(os.Stderr, ui.ErrorCard(err.Error()))
			} else {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
			return err
		}

		fullText.WriteString(resp.Text)
		if isInteractive {
			banner := ui.HeaderBanner(resp.Provider, resp.Model, resp.UsedLocal)
			rendered, _ := ui.RenderMarkdown(resp.Text, ui.TerminalWidth())
			fmt.Printf("%s\n\n%s", banner, rendered)
		} else {
			fmt.Print(resp.Text)
			if !strings.HasSuffix(resp.Text, "\n") {
				fmt.Println()
			}
		}
	} else {
		// Streaming output
		stream, err := r.GenerateStream(ctx, req)
		if err != nil {
			fmt.Fprintln(os.Stderr, ui.ErrorCard(err.Error()))
			return err
		}

		for chunk := range stream {
			if chunk.Error != nil {
				fmt.Fprintf(os.Stderr, "\n[stream error]: %v\n", chunk.Error)
				return chunk.Error
			}
			fmt.Print(chunk.Text)
			fullText.WriteString(chunk.Text)
		}
		if !strings.HasSuffix(fullText.String(), "\n") {
			fmt.Println()
		}
	}

	// 7. Check for proposed shell command and handle execution review
	return handleProposedAction(ctx, cfg, fullText.String(), isInteractive)
}

func handleProposedAction(ctx context.Context, cfg *config.Config, responseText string, isInteractive bool) error {
	cmdStr := shell.ExtractProposedCommand(responseText)
	if cmdStr == "" {
		return nil
	}

	isDestructive, reason := shell.IsDestructiveCommand(cmdStr)
	yolo := flagYoloApproveAll || cfg.Shell.YoloApproveAll
	autoAccept := flagAutoAccept || cfg.Shell.AutoAccept

	// 1. YOLO Mode: Auto-execute everything immediately with zero prompts
	if yolo {
		if isInteractive {
			fmt.Println()
			fmt.Println(ui.CommandCard("Auto-Executing Command (--yolo-approve-all)", cmdStr))
		}
		return shell.ExecuteInActiveShell(ctx, cmdStr)
	}

	// 2. Auto-Accept Mode: Auto-execute non-destructive commands
	if autoAccept && !isDestructive {
		if isInteractive {
			fmt.Println()
			fmt.Println(ui.CommandCard("Auto-Executing Safe Command (--auto-accept)", cmdStr))
		}
		return shell.ExecuteInActiveShell(ctx, cmdStr)
	}

	// Non-interactive pipelines without YOLO/AutoAccept should not prompt
	if !isInteractive {
		if isDestructive {
			return fmt.Errorf("command is destructive (%s); use --yolo-approve-all to execute in non-interactive mode", reason)
		}
		return nil
	}

	// 3. Interactive Destructive Guard: Requires typed confirmation
	if isDestructive {
		fmt.Println()
		fmt.Println(ui.Card(
			ui.BadgeWarning("Destructive Command Guard"),
			fmt.Sprintf("Proposed Command:\n  %s\n\nRisk: %s", cmdStr, reason),
			"Requires typed confirmation before execution",
		))

		confirmed, err := ui.PromptDestructiveConfirm("Warning: This command may perform destructive modifications.", "yes-execute")
		if err != nil || !confirmed {
			fmt.Println("Execution cancelled.")
			return nil
		}
		return shell.ExecuteInActiveShell(ctx, cmdStr)
	}

	// 4. Interactive Review: [Run] [Edit] [Cancel]
	fmt.Println()
	fmt.Println(ui.CommandCard("Proposed Shell Command", cmdStr))

	action, finalCmd, err := ui.PromptCommandReview(cmdStr)
	if err != nil || action == ui.ActionCancel {
		return nil
	}

	if action == ui.ActionRun {
		return shell.ExecuteInActiveShell(ctx, finalCmd)
	}

	return nil
}
