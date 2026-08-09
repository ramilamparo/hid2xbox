package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	setup := flag.Bool("setup", false, "Run the interactive TUI to discover inputs and create a config")
	config := flag.String("config", "config.json", "Path to the JSON configuration file")
	flag.Parse()

	if *setup {
		if err := runTUI(*config); err != nil {
			fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	ctx := signalContext()
	if err := RunBridge(ctx, *config); err != nil {
		fmt.Fprintf(os.Stderr, "Bridge error: %v\n", err)
		os.Exit(1)
	}
}
