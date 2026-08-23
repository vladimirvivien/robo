package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/daemon"
	"github.com/vladimirvivien/robo/internal/engine"
	"github.com/vladimirvivien/robo/internal/engine/cloud"
	"github.com/vladimirvivien/robo/internal/engine/local"
	"github.com/vladimirvivien/robo/internal/ui"
)

var (
	cfgFile        string
	flagOutput     string
	flagSystem     string
	flagAutoAccept bool
	flagOneShot    bool
	flagDryRun     bool
	flagMaxSteps   int
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
	RootCmd.Flags().StringVar(&flagSystem, "system", "", "custom system prompt override")
	RootCmd.Flags().BoolVarP(&flagAutoAccept, "yolo", "y", false, "auto-accept all non-destructive actions without prompt")
	RootCmd.Flags().BoolVarP(&flagOneShot, "one-shot", "1", false, "force strictly single-turn execution (N=1) without follow-up loop")
	RootCmd.Flags().BoolVarP(&flagDryRun, "dry-run", "d", false, "simulate execution plan without host mutation")
	RootCmd.Flags().IntVar(&flagMaxSteps, "max-steps", 5, "maximum number of agent completion steps")
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
		if cfgFile != "" {
			return fmt.Errorf("configuration file not found: %s\n\nTo resolve:\n  • Specify a valid configuration file with --config\n  • Or run 'robo init' to initialize robo", cfgFile)
		}
		return fmt.Errorf("robo is not initialized (configuration not found at %s)\n\nTo resolve:\n  • Run 'robo init' to set up local models and configuration", targetCfgPath)
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Apply CLI flags to config overrides
	if flagAutoAccept {
		cfg.Robo.AutoAccept = true
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
	if !cmd.Flags().Changed("output") && cfg.Robo.OutputMode != "" {
		outputFormat = cfg.Robo.OutputMode
	}
	cfg.Robo.OutputMode = outputFormat

	if _, err := ui.NewFormatter(outputFormat, false, 80); err != nil {
		return err
	}

	// 3. Validate inference environment setup
	if err := engine.ValidateInferenceSetup(cfg, cfg.Robo.InferenceMode); err != nil {
		return err
	}

	// 4. Start visual spinner immediately upon prompt receipt
	isInteractive := (ui.IsStdoutTerminal() || ui.IsStderrTerminal()) && (outputFormat == "markdown" || outputFormat == "md" || outputFormat == "")
	if isInteractive {
		ui.StartSpinner("Initializing...")
	}
	defer ui.StopActiveSpinner()

	// 5. Construct engine based on inference_mode
	var execEngine engine.Engine
	var providerName, modelName string
	var usedLocal bool

	switch strings.ToLower(cfg.Robo.InferenceMode) {
	case "llm", "cloud":
		cloudEngine := cloud.New(cfg.LLM, cfg)
		defer func() { _ = cloudEngine.Close() }()
		execEngine = cloudEngine
		providerName = cfg.LLM.Provider
		modelName = cfg.LLM.Model
		usedLocal = false

	case "auto":
		// Placeholder: defaults to local SLM execution
		inProcEngine := local.New(cfg.SLM, cfg)
		localClient := daemon.NewClient(*cfg, daemon.WithInProcEngine(inProcEngine))
		defer func() { _ = localClient.Close() }()
		execEngine = localClient
		providerName = "litertlm"
		modelName = cfg.SLM.Model
		usedLocal = true

	case "slm", "local", "":
		fallthrough
	default:
		inProcEngine := local.New(cfg.SLM, cfg)
		localClient := daemon.NewClient(*cfg, daemon.WithInProcEngine(inProcEngine))
		defer func() { _ = localClient.Close() }()
		execEngine = localClient
		providerName = "litertlm"
		modelName = cfg.SLM.Model
		usedLocal = true
	}

	sessionConfig := engine.SessionConfig{
		MaxSteps:           flagMaxSteps,
		OneShot:            flagOneShot,
		Yolo:               flagAutoAccept || cfg.Robo.AutoAccept || cfg.Robo.YoloApproveAll,
		DryRun:             flagDryRun,
		OutputFormat:       outputFormat,
		ForceBackend:       cfg.Robo.InferenceMode,
		CustomInstructions: flagSystem,
		StdinContent:       stdinContent,
	}

	runner := engine.NewSessionRunner(execEngine, cfg, sessionConfig)
	res, err := runner.Run(ctx, prompt)
	if err != nil {
		return err
	}

	sessionSteps := make([]ui.TrajectoryStep, len(res.Steps))
	for i, s := range res.Steps {
		sessionSteps[i] = ui.TrajectoryStep{
			Step:        s.Step,
			Command:     s.Command,
			Description: s.Description,
			Output:      s.Output,
			Error:       s.Error,
			ExitCode:    s.ExitCode,
			Executed:    s.Executed,
			RiskTier:    s.RiskTier,
			RiskScore:   s.RiskScore,
		}
	}

	sessionData := ui.SessionOutputData{
		Goal:          res.Goal,
		Status:        res.Status,
		TotalSteps:    res.TotalSteps,
		Steps:         sessionSteps,
		FinalResponse: res.FinalResponse,
		Provider:      providerName,
		Model:         modelName,
		Local:         usedLocal,
	}

	trajFormatter := ui.NewTrajectoryFormatter(outputFormat, isInteractive, ui.TerminalWidth())
	return trajFormatter.FormatSession(os.Stdout, sessionData)
}
