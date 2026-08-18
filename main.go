package main

import (
	"os"

	"github.com/vladimirvivien/robo/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
