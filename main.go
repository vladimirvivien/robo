package main

import (
	"io"
	"log"
	"log/slog"
	"os"

	"github.com/vladimirvivien/robo/cmd"
)

func init() {
	_ = os.Setenv("GENKIT_LOG_LEVEL", "warn")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	log.SetOutput(io.Discard)
}

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
