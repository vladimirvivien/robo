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
	Use:   "robo [prompt]",
	Short: "robo: AI-native developer companion and terminal assistant",
	Long: `robo is a two-tier AI assistant with sub-50ms hot-start on-device execution
powered by LiteRT-LM (robod) and intelligent automatic escalation to frontier cloud models.`,
	Args:          cobra.ArbitraryArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runRoot,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return RootCmd.Execute()
}

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	log.SetOutput(io.Discard)

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.config/robo/config.yaml)")
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

	// 1. Check if first-time onboarding is needed (no config file on disk and no model downloaded)
	configExists := config.ConfigFileExists(cfgFile)
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if !configExists && !engine.IsModelDownloaded(cfg.LLM.Local.Model) {
		if err := engine.RunInitialSetup(ctx, cfg); err != nil {
			return err
		}
	}

	// 2. Read prompt and stdin
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
		if ui.IsStdoutTerminal() && ui.IsStdinTerminal() {
			return runChat(cmd, args)
		}
		return cmd.Help()
	}

	// 3. Resolve effective output format
	outputFormat := flagOutput
	if !cmd.Flags().Changed("output") && cfg.Shell.OutputMode != "" {
		outputFormat = cfg.Shell.OutputMode
	}

	if _, err := ui.NewFormatter(outputFormat, false, 80); err != nil {
		return err
	}

	// 4. Start visual spinner immediately upon one-shot prompt receipt
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

	// 4. Validate inference environment setup
	forceBackend := ""
	if flagLocal {
		forceBackend = "local-only"
	} else if flagCloud {
		forceBackend = "cloud-only"
	}
	if err := engine.ValidateInferenceSetup(cfg, forceBackend); err != nil {
		if sp != nil {
			sp.Stop()
		}
		if ui.IsStdoutTerminal() {
			fmt.Fprintln(os.Stderr, ui.ErrorCard(err.Error()))
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		return err
	}

	// 5. Assemble ambient shell context (pure in-memory / local files)
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
			maxLines = 5
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

	stream, err := r.GenerateStream(ctx, req)
	if err != nil {
		if sp != nil {
			sp.Stop()
		}
		if isInteractive {
			fmt.Fprintln(os.Stderr, ui.ErrorCard(err.Error()))
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
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
	}
	if sp != nil {
		sp.Stop()
	}

	rawResponse := fullText.String()
	cleaned := ui.CleanResponseText(rawResponse)
	cmdStr := shell.ExtractProposedCommand(cleaned)
	explanation := strings.TrimSpace(shell.StripCodeBlock(cleaned))

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
		Provider:    providerName,
		Model:       modelName,
		Local:       usedLocal,
	}

	return formatter.Format(os.Stdout, outputData)
}
