package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vladimirvivien/robo/internal/shell"
)

// DoCmd represents the "robo do" shell assistant command.
var DoCmd = &cobra.Command{
	Use:   "do [task description]",
	Short: "Synthesize and execute a shell command for a natural-language task",
	Long: `Translates natural-language requests into executable commands tailored for your
active shell (Bash, Zsh, Fish, or PowerShell) and provides interactive execution controls.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		shellType := shell.DetectShell()
		instruction := fmt.Sprintf(
			"You are an expert CLI assistant. Synthesize a single, correct, and optimal shell command for the user's active shell (%s).\n"+
				"Format your response as a single markdown code block:\n```%s\n<command>\n```\n"+
				"Do not include conversational preamble before or after the code block unless a brief warning is essential.",
			shellType, shellType,
		)

		flagSystem = instruction
		return runRoot(cmd, args)
	},
}

func init() {
	DoCmd.Flags().BoolVarP(&flagLocal, "local-only", "l", false, "force execution on local on-device SLM")
	DoCmd.Flags().BoolVarP(&flagCloud, "cloud-only", "c", false, "force execution on cloud frontier model")
	DoCmd.Flags().BoolVarP(&flagAutoAccept, "auto-accept", "y", false, "auto-accept all non-destructive actions without prompt")
	DoCmd.Flags().BoolVar(&flagYoloApproveAll, "yolo-approve-all", false, "auto-accept and execute all actions including destructive ones (no prompts)")
	RootCmd.AddCommand(DoCmd)
}
