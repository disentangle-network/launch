package main

import (
	"os"

	"github.com/disentangle-network/launch/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
