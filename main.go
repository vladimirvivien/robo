package main

import (
	"fmt"
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
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
