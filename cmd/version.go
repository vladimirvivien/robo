package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vladimirvivien/robo/internal/ui"
)

var (
	// Version is populated at build time via -ldflags.
	Version = "dev"
	// Commit is populated at build time via -ldflags.
	Commit = "none"
	// BuildDate is populated at build time via -ldflags.
	BuildDate = "unknown"
)

var flagVersionJSON bool

// BuildInfo represents runtime and build version metadata.
type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// GetBuildInfo returns the current build and runtime metadata.
func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// VersionCmd represents the "robo version" subcommand.
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print build and version information",
	Long:  `Displays version string, Git commit SHA, build timestamp, and runtime platform.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		info := GetBuildInfo()
		outMode := strings.ToLower(strings.TrimSpace(flagOutput))

		if flagVersionJSON || outMode == "json" {
			data, err := json.MarshalIndent(info, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		if ui.IsStdoutTerminal() && (outMode == "markdown" || outMode == "md" || outMode == "") {
			content := fmt.Sprintf("Version:    %s\nCommit:     %s\nBuild Date: %s\nGo Version: %s\nPlatform:   %s",
				info.Version, info.Commit, info.BuildDate, info.GoVersion, info.Platform)
			fmt.Println(ui.Card("Robo Build Information", content, ""))
			return nil
		}

		fmt.Printf("robo version %s (commit %s, built %s, %s)\n",
			info.Version, info.Commit, info.BuildDate, info.Platform)
		return nil
	},
}

func init() {
	VersionCmd.Flags().BoolVar(&flagVersionJSON, "json", false, "output version information in JSON format")
	RootCmd.AddCommand(VersionCmd)
}
