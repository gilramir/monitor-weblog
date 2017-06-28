// Copyright (c) 2017 by Gilbert Ramirez <gram@alumni.rice.edu>
package main

import (
	"context"
	"fmt"
	"github.com/gilramir/argparse"
	"github.com/gilramir/monitor-weblog/collator"
	"github.com/pkg/errors"
	"os"
)

// These hold the values from the command line.
type Options struct {
	Filename       string
	AlertThreshold int
}

func main() {
	// Create the argument parser
	argumentParser := &argparse.ArgumentParser{
		Name:             "monitor web-log",
		ShortDescription: "Monitor logs in the Common Log format",
		Destination:      &Options{},
	}

	// First positional argument
	argumentParser.AddArgument(&argparse.Argument{
		Name: "filename",
		Help: "The log file to monitor",
	})

	// Second positional argument
	argumentParser.AddArgument(&argparse.Argument{
		Name: "alertThreshold",
		Help: "The number of hits per second at which to alert",
	})

	// Parse the CLI, and run it.
	err := argumentParser.ParseArgs()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Run the text UI, and report any error that happened
func (self *Options) Run(parents []argparse.Destination) error {
	// Ensure the file exists before starting.
	if _, err := os.Stat(self.Filename); err != nil {
		return errors.Errorf("Cannot read %s", self.Filename)
	}

	// Start the Collator
	ctx, cancelFunc := context.WithCancel(context.Background())
	c, err := collator.NewAndRun(ctx, self.Filename, self.AlertThreshold)
	if err != nil {
		return err
	}

	// Run the UI; this returns when the UI stops.
	err = runUI(cancelFunc, c)
	if err != nil {
		return err
	}

	return nil
}
