package main

import (
	"os"

	"github.com/ggh41th/kubeforen/cmd/controller-manager/app"
)

func main() {
	cmd := app.NewControllerManagerCommand()

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
