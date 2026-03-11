package cmd

import (
	"os"

)

func main() {
	cmd := app.NewControllerManagerCommand()

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
