package main

import (
	"os"

	"github.com/mattsolo1/grove-docgen/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
