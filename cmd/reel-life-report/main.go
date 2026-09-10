package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/patflynn/reel-life/internal/evaluation"
)

func main() {
	path := flag.String("events", "events.jsonl", "path to the product evidence ledger")
	flag.Parse()
	f, err := os.Open(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open evidence ledger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = f.Close() }()
	report, err := evaluation.Build(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build report: %v\n", err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		os.Exit(1)
	}
}
