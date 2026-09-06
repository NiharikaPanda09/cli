package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

func main() {
	in := flag.String("in", "-", "packet JSON to read, or - for stdin")
	dryRun := flag.Bool("dry-run", false, "print the rows that would be sent and exit")
	flag.Parse()

	if err := run(context.Background(), *in, *dryRun, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "handoff-databricks:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, in string, dryRun bool, stdout, stderr io.Writer) error {
	raw, err := readInput(in)
	if err != nil {
		return err
	}

	var p Packet
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("parse packet: %w", err)
	}

	now := time.Now().UTC()
	rows := Flatten(p, now)
	if len(rows) == 0 {
		fmt.Fprintln(stderr, "packet contained no sections; nothing to ingest")
		return nil
	}

	payload, err := EncodeNDJSON(rows)
	if err != nil {
		return fmt.Errorf("encode rows: %w", err)
	}

	if dryRun {
		_, err := stdout.Write(payload)
		return err
	}

	cfg, err := ConfigFromEnv()
	if err != nil {
		return err
	}

	u := NewUploader(cfg)
	dest, err := u.Upload(ctx, p, payload, now)
	if err != nil {
		return err
	}
	if err := u.CopyInto(ctx, dest); err != nil {
		return err
	}

	fmt.Fprintf(stderr, "ingested %d rows for %s -> %s\n", len(rows), p.Repo, dest)
	return nil
}

func readInput(in string) ([]byte, error) {
	if in == "-" || in == "" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(in)
}
