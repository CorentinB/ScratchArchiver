package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/akamensky/argparse"
)

var arguments = struct {
	OutputDir   string
	Proxy       string
	Concurrency int
}{
	// Default arguments
	Concurrency: 4,
	Proxy:       "",
}

func argumentParsing(args []string) {
	// Create new parser object
	parser := argparse.NewParser("ScratchArchiver", "Let's archive scratch.mit.edu, shall we?")

	output := parser.String("o", "output", &argparse.Options{
		Required: true,
		Help:     "Output directory"})
	proxy := parser.String("p", "proxy", &argparse.Options{
		Required: false,
		Help:     "Proxy"})
	concurrency := parser.Int("w", "workers", &argparse.Options{
		Required: false,
		Help:     "Parallel workers to run"})

	// Parse input
	err := parser.Parse(args)
	if err != nil {
		// In case of error print error and print usage
		// This can also be done by passing -h or --help flags
		fmt.Print(parser.Usage(err))
		os.Exit(0)
	}

	// Convert path parameters to absolute paths
	outputDir, err := filepath.Abs(*output)
	if err != nil {
		log.Fatal(err)
	}

	// Finally save the collected flags
	arguments.OutputDir = outputDir
	arguments.Proxy = *proxy
	arguments.Concurrency = *concurrency
}
