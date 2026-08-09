package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
)

type configsFlag []string

func (c *configsFlag) String() string { return strings.Join(*c, ", ") }
func (c *configsFlag) Set(v string) error {
	*c = append(*c, v)
	return nil
}

func main() {
	var configs configsFlag
	setup := flag.Bool("setup", false, "Run the interactive TUI to discover inputs and create a config")
	debugLog := flag.String("debug-log", "", "Write debug logs (inputs, values, errors) to file")
	debugStdout := flag.Bool("debug", false, "Show virtual Xbox 360 controller state in real time")
	flag.Var(&configs, "config", "Path to JSON config file (repeatable)")
	flag.Parse()

	if len(configs) == 0 {
		configs = configsFlag{"config.json"}
	}

	if *setup {
		if err := runTUI(configs[0]); err != nil {
			fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	ctx := signalContext()
	var wg sync.WaitGroup
	var connectMu sync.Mutex
	errs := make(chan error, len(configs))

	for i, cfg := range configs {
		wg.Add(1)
		go func(id int, path string) {
			defer wg.Done()
			if err := RunBridge(ctx, path, &connectMu, *debugLog, *debugStdout); err != nil {
				if errors.Is(err, context.Canceled) {
					fmt.Printf("Controller %d (%s): shutting down\n", id+1, path)
				} else {
					errs <- fmt.Errorf("controller %d (%s): %w", id+1, path, err)
				}
			}
		}(i, cfg)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		fmt.Fprintf(os.Stderr, "Bridge error: %v\n", err)
		os.Exit(1)
	}
}
