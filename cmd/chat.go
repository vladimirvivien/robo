package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/daemon"
	"github.com/vladimirvivien/robo/internal/engine"
	"github.com/vladimirvivien/robo/internal/engine/cloud"
	"github.com/vladimirvivien/robo/internal/engine/local"
	"github.com/vladimirvivien/robo/internal/engine/router"
	"github.com/vladimirvivien/robo/internal/history"
	"github.com/vladimirvivien/robo/internal/shell"
	"github.com/vladimirvivien/robo/internal/ui"
)

var (
	flagChatSession string
)

// ChatCmd represents the interactive REPL conversation command.
var ChatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive multi-turn conversation session",
	Long: `Starts a persistent multi-turn conversational REPL in the terminal.
Maintains history across sessions using local SQLite storage (~/.config/robo/history.db).`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runChat,
}

func init() {
	ChatCmd.Flags().StringVarP(&flagChatSession, "session", "s", "", "session name to open or create")
	ChatCmd.Flags().BoolVarP(&flagLocal, "local-only", "l", false, "force execution on local on-device SLM")
	ChatCmd.Flags().BoolVarP(&flagCloud, "cloud-only", "c", false, "force execution on cloud frontier model")
	ChatCmd.Flags().BoolVarP(&flagAutoAccept, "auto-accept", "y", false, "auto-accept all non-destructive actions without prompt")
	ChatCmd.Flags().BoolVar(&flagYoloApproveAll, "yolo-approve-all", false, "auto-accept and execute all actions including destructive ones (no prompts)")
	RootCmd.AddCommand(ChatCmd)
}

func runChat(cmd *cobra.Command, args []string) error {
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

	// 2. Open SQLite history store
	store, err := history.NewStore("")
	if err != nil {
		return fmt.Errorf("open history store: %w", err)
	}
	defer func() { _ = store.Close() }()

	// 3. Resolve active session
	var currentSession *history.Session
	if flagChatSession != "" {
		currentSession, err = store.GetSessionByName(ctx, flagChatSession)
		if err != nil {
			currentSession, err = store.CreateSession(ctx, flagChatSession, "thread")
			if err != nil {
				return fmt.Errorf("create session: %w", err)
			}
		}
	} else {
		currentSession, err = store.GetOrCreateDailySession(ctx)
		if err != nil {
			return fmt.Errorf("get daily session: %w", err)
		}
	}

	// 4. Initialize Engines
	inProcEngine := local.New(cfg.LLM.Local)
	localClient := daemon.NewClient(*cfg, daemon.WithInProcEngine(inProcEngine))
	cloudEngine := cloud.New(cfg.LLM.Cloud)

	r := router.NewRouter(localClient, cloudEngine, cfg.LLM)
	defer func() { _ = r.Close() }()

	// Current model routing override for this chat session
	activeBackend := ""
	if flagLocal {
		activeBackend = "local-only"
	} else if flagCloud {
		activeBackend = "cloud-only"
	}

	// 5. Pre-flight inference environment validation
	if err := engine.ValidateInferenceSetup(cfg, activeBackend); err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrorCard(err.Error()))
		return err
	}

	// 6. Welcome Banner
	welcomeText := fmt.Sprintf(
		"Session: %s • Mode: %s\nType /help for slash commands, /model to switch backend, /exit to quit",
		currentSession.Name, currentSession.Mode,
	)
	fmt.Println(ui.Card("robo chat v0.1.0", welcomeText, "Multi-turn history backed by SQLite"))

	scanner := bufio.NewScanner(os.Stdin)

	// 6. Interactive REPL Loop
	for {
		fmt.Print(ui.PromptIndicator(currentSession.Name))

		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Handle Slash Commands
		if strings.HasPrefix(line, "/") {
			handled, exit := handleSlashCommand(ctx, line, store, &currentSession, &activeBackend, cfg)
			if exit {
				fmt.Println("Goodbye!")
				return nil
			}
			if handled {
				continue
			}
		}

		// Start spinner immediately so the user has immediate visual feedback
		sp := ui.StartSpinner("Working...")

		// Fetch recent conversation history from SQLite (sliding window)
		recentMsgs, _ := store.GetMessages(ctx, currentSession.ID, 10)
		var convContext strings.Builder
		if len(recentMsgs) > 0 {
			convContext.WriteString("Previous conversation turns:\n")
			for _, m := range recentMsgs {
				fmt.Fprintf(&convContext, "%s: %s\n", strings.ToUpper(m.Role), m.Content)
			}
			convContext.WriteString("\n")
		}

		// Build ambient context with default persona and runtime target
		var systemPrompt strings.Builder
		systemPrompt.WriteString(config.DefaultRoboSystemPrompt)
		systemPrompt.WriteString("\n\n")

		// Inject OS / Architecture / Shell environment target
		fmt.Fprintf(&systemPrompt, "[Runtime Target]\nOS: %s\nArchitecture: %s\nActive Shell: %s\n\n", runtime.GOOS, runtime.GOARCH, shell.DetectShell())

		if cfg.Shell.CaptureHistory {
			collector := shell.NewCollector(nil)
			sc, err := collector.Collect(ctx, 3)
			if err == nil && sc != nil {
				systemPrompt.WriteString(sc.FormatPromptContext())
				systemPrompt.WriteString("\n")
			}
		}
		if convContext.Len() > 0 {
			systemPrompt.WriteString(convContext.String())
		}

		req := engine.Request{
			Prompt:       line,
			SystemPrompt: systemPrompt.String(),
			ForceBackend: activeBackend,
		}

		// Save user turn in SQLite
		if _, err := store.AppendMessage(ctx, currentSession.ID, history.Message{
			Role:      "user",
			Content:   line,
			CreatedAt: time.Now().UTC(),
		}); err != nil {
			fmt.Fprintf(os.Stderr, "history warning: %v\n", err)
		}

		stream, err := r.GenerateStream(ctx, req)
		if err != nil {
			sp.Stop()
			fmt.Fprintln(os.Stderr, ui.ErrorCard(err.Error()))
			continue
		}

		var fullResponse strings.Builder
		tokensUsed := 0
		firstChunk := true

		for chunk := range stream {
			if firstChunk {
				sp.Stop()
				firstChunk = false
			}
			if chunk.Error != nil {
				fmt.Fprintf(os.Stderr, "\n[stream error]: %v\n", chunk.Error)
				break
			}
			fmt.Print(chunk.Text)
			fullResponse.WriteString(chunk.Text)
			if chunk.TokensUsed > 0 {
				tokensUsed = chunk.TokensUsed
			}
		}
		sp.Stop()
		if !strings.HasSuffix(fullResponse.String(), "\n") {
			fmt.Println()
		}

		// Save assistant turn in SQLite
		usedLocal := activeBackend == "local-only" || (activeBackend == "" && cfg.LLM.Local.Enabled)
		providerName := cfg.LLM.Local.Provider
		modelName := cfg.LLM.Local.Model
		if !usedLocal {
			providerName = cfg.LLM.Cloud.Provider
			modelName = cfg.LLM.Cloud.Model
		}

		if _, err := store.AppendMessage(ctx, currentSession.ID, history.Message{
			Role:       "assistant",
			Content:    fullResponse.String(),
			Provider:   providerName,
			Model:      modelName,
			TokensUsed: tokensUsed,
			CreatedAt:  time.Now().UTC(),
		}); err != nil {
			fmt.Fprintf(os.Stderr, "history warning: %v\n", err)
		}

		// Check and handle proposed actions / commands
		if err := handleProposedAction(ctx, cfg, fullResponse.String(), true); err != nil {
			fmt.Fprintf(os.Stderr, "action error: %v\n", err)
		}
	}

	return scanner.Err()
}

func handleSlashCommand(
	ctx context.Context,
	input string,
	store *history.Store,
	currentSession **history.Session,
	activeBackend *string,
	cfg *config.Config,
) (handled bool, exit bool) {
	parts := strings.Fields(input)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/exit", "/quit", "/q":
		return true, true

	case "/help", "/h", "/?":
		helpContent := `Available Slash Commands:
  /model [local|cloud|auto]   Switch routing backend or view active model
  /session <name>             Switch to or create a named conversation session
  /sessions                   List recent saved conversation sessions
  /clear                      Clear the terminal screen
  /save [filepath]            Export active conversation history to a Markdown file
  /help                       Show this help reference
  /exit, /quit                Exit the chat session`
		fmt.Println(ui.Card("Slash Command Reference", helpContent, ""))
		return true, false

	case "/model":
		if len(parts) < 2 {
			currentMode := *activeBackend
			if currentMode == "" {
				currentMode = "auto"
			}
			fmt.Printf("Active Backend: %s\nLocal SLM:      %s (%s)\nCloud LLM:      %s (%s)\n",
				currentMode, cfg.LLM.Local.Model, cfg.LLM.Local.Provider, cfg.LLM.Cloud.Model, cfg.LLM.Cloud.Provider)
			return true, false
		}

		target := strings.ToLower(parts[1])
		switch target {
		case "local", "local-only", "l":
			if err := engine.ValidateInferenceSetup(cfg, "local-only"); err != nil {
				fmt.Println(ui.ErrorCard(err.Error()))
				return true, false
			}
			*activeBackend = "local-only"
			fmt.Printf("Routing backend set to: %s\n", ui.BadgeLocal("local-only: "+cfg.LLM.Local.Model))
		case "cloud", "cloud-only", "c":
			if err := engine.ValidateInferenceSetup(cfg, "cloud-only"); err != nil {
				fmt.Println(ui.ErrorCard(err.Error()))
				return true, false
			}
			*activeBackend = "cloud-only"
			fmt.Printf("Routing backend set to: %s\n", ui.BadgeCloud("cloud-only: "+cfg.LLM.Cloud.Model))
		case "auto", "a":
			*activeBackend = ""
			fmt.Printf("Routing backend restored to: %s\n", ui.BadgeCloud("auto (hybrid)"))
		default:
			fmt.Printf("Unknown model backend %q. Use: /model local, /model cloud, or /model auto\n", target)
		}
		return true, false

	case "/session":
		if len(parts) < 2 {
			fmt.Printf("Current session: %s (mode: %s)\n", (*currentSession).Name, (*currentSession).Mode)
			return true, false
		}
		name := parts[1]
		sess, err := store.GetSessionByName(ctx, name)
		if err != nil {
			sess, err = store.CreateSession(ctx, name, "thread")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating session %q: %v\n", name, err)
				return true, false
			}
			fmt.Printf("Created and switched to new session: %q\n", name)
		} else {
			msgs, _ := store.GetMessages(ctx, sess.ID, 0)
			fmt.Printf("Switched to session: %q (%d previous messages loaded)\n", name, len(msgs))
		}
		*currentSession = sess
		return true, false

	case "/sessions":
		sessions, err := store.ListSessions(ctx, 15)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing sessions: %v\n", err)
			return true, false
		}
		var sb strings.Builder
		for i, s := range sessions {
			activeMark := " "
			if s.ID == (*currentSession).ID {
				activeMark = "▶"
			}
			fmt.Fprintf(&sb, "%s %d. %-24s (mode: %-6s, updated: %s)\n",
				activeMark, i+1, s.Name, s.Mode, s.UpdatedAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Println(ui.Card("Stored Sessions", sb.String(), "Use '/session <name>' to switch"))
		return true, false

	case "/clear":
		fmt.Print("\033[H\033[2J")
		return true, false

	case "/save":
		filePath := fmt.Sprintf("robo-chat-%s.md", (*currentSession).Name)
		if len(parts) >= 2 {
			filePath = parts[1]
		}
		msgs, err := store.GetMessages(ctx, (*currentSession).ID, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error retrieving messages: %v\n", err)
			return true, false
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "# Robo Chat Session: %s\n\n", (*currentSession).Name)
		fmt.Fprintf(&sb, "- **Date:** %s\n- **Mode:** %s\n\n---\n\n", (*currentSession).StartedAt.Format("2006-01-02 15:04:05"), (*currentSession).Mode)
		for _, m := range msgs {
			fmt.Fprintf(&sb, "### %s (%s)\n\n%s\n\n", strings.ToUpper(m.Role), m.CreatedAt.Format("15:04:05"), m.Content)
		}

		if err := os.WriteFile(filePath, []byte(sb.String()), 0600); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving transcript to %s: %v\n", filePath, err)
			return true, false
		}
		absPath, _ := filepath.Abs(filePath)
		fmt.Printf("Exported session transcript to %s\n", absPath)
		return true, false
	}

	return false, false
}
