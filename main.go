package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/codemob-ai/codemob/cmd"
	"github.com/codemob-ai/codemob/internal/prompt"
)

func main() {
	if err := cmd.Execute(); err != nil {
		if errors.Is(err, prompt.ErrCancelled) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
