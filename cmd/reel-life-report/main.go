package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/patflynn/reel-life/internal/evaluation"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("reel-life-report", flag.ContinueOnError)
	path := flags.String("events", "events.jsonl", "path to the product evidence ledger")
	summary := flags.Bool("summary", false, "emit counters and timestamps only")
	maxBytes := flags.Int64("max-bytes", 64*1024*1024, "maximum ledger size to read")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *maxBytes <= 0 {
		return fmt.Errorf("unexpected arguments or invalid size limit")
	}
	f, err := os.Open(*path)
	if err != nil {
		return fmt.Errorf("open evidence ledger: %w", err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat evidence ledger: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("evidence ledger must be a regular file")
	}
	if info.Size() > *maxBytes {
		return fmt.Errorf("evidence ledger exceeds %d-byte report limit", *maxBytes)
	}
	// Read the size observed at open time; concurrent appends cannot make this
	// invocation run indefinitely. A partial trailing event is counted malformed.
	report, err := evaluation.Build(io.NewSectionReader(f, 0, info.Size()))
	if err != nil {
		return fmt.Errorf("build report: %w", err)
	}
	var result any = report
	if *summary {
		result = report.Summary(info.Size(), time.Now())
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}
