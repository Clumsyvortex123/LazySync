package main

import (
	"fmt"
	"os"

	"lazysync/pkg/app"
)

func main() {
	app, err := app.NewApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing app: %v\n", err)
		os.Exit(1)
	}

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running app: %v\n", err)
		os.Exit(1)
	}
}
