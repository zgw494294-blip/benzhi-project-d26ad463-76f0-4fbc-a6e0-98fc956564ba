package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"patchproof/patchproof"
)

const defaultLedgerPath = ".patchproof.json"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	ledgerPath, command, commandArgs, err := parseGlobal(args)
	if err != nil {
		return reportError(stderr, err)
	}
	if command == "" || command == "help" || command == "--help" || command == "-h" {
		printUsage(stdout)
		return 0
	}
	var commandErr error
	switch command {
	case "open":
		commandErr = runOpen(ledgerPath, commandArgs, stdout, stderr)
	case "connect":
		commandErr = runConnect(ledgerPath, commandArgs, stdout, stderr)
	case "activate":
		commandErr = runActivate(ledgerPath, commandArgs, stdout, stderr)
	case "close":
		commandErr = runClose(ledgerPath, commandArgs, stdout, stderr)
	case "show":
		commandErr = runShow(ledgerPath, commandArgs, stdout, stderr)
	case "smoke":
		commandErr = runSmoke(stdout, stderr)
	default:
		commandErr = fmt.Errorf("unknown command %q", command)
	}
	if commandErr != nil {
		return reportError(stderr, commandErr)
	}
	return 0
}

func parseGlobal(args []string) (string, string, []string, error) {
	if len(args) == 0 {
		return "", "", nil, errors.New("a command is required")
	}
	ledgerPath := defaultLedgerPath
	if args[0] == "--ledger" {
		if len(args) < 2 || args[1] == "" {
			return "", "", nil, errors.New("--ledger requires a path")
		}
		ledgerPath = args[1]
		args = args[2:]
		if len(args) == 0 {
			return "", "", nil, errors.New("a command is required")
		}
	}
	return ledgerPath, args[0], args[1:], nil
}

func runOpen(path string, args []string, stdout, stderr io.Writer) error {
	flags := newFlags("open", stderr)
	id := flags.String("id", "", "unique plan identifier")
	note := flags.String("note", "", "optional operator note")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("open does not accept positional arguments")
	}
	var plan patchproof.Plan
	err := mutate(path, func(ledger *patchproof.Ledger) error {
		var err error
		plan, err = ledger.Open(*id, *note)
		return err
	})
	if err != nil {
		return err
	}
	return writeJSON(stdout, plan)
}

func runConnect(path string, args []string, stdout, stderr io.Writer) error {
	flags := newFlags("connect", stderr)
	planID := flags.String("id", "", "plan identifier")
	source := flags.String("source", "", "source endpoint")
	destination := flags.String("destination", "", "destination endpoint")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("connect does not accept positional arguments")
	}
	var connection patchproof.Connection
	err := mutate(path, func(ledger *patchproof.Ledger) error {
		var err error
		connection, err = ledger.Connect(*planID, *source, *destination)
		return err
	})
	if err != nil {
		return err
	}
	return writeJSON(stdout, connection)
}

func runActivate(path string, args []string, stdout, stderr io.Writer) error {
	flags := newFlags("activate", stderr)
	planID := flags.String("id", "", "plan identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("activate does not accept positional arguments")
	}
	var plan patchproof.Plan
	err := mutate(path, func(ledger *patchproof.Ledger) error {
		if err := ledger.Activate(*planID); err != nil {
			return err
		}
		var ok bool
		plan, ok = ledger.Plan(*planID)
		if !ok {
			return errors.New("activated plan disappeared")
		}
		return nil
	})
	if err != nil {
		return err
	}
	return writeJSON(stdout, plan)
}

func runClose(path string, args []string, stdout, stderr io.Writer) error {
	flags := newFlags("close", stderr)
	planID := flags.String("id", "", "plan identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("close does not accept positional arguments")
	}
	var receipt patchproof.Receipt
	err := mutate(path, func(ledger *patchproof.Ledger) error {
		var err error
		receipt, err = ledger.Close(*planID)
		return err
	})
	if err != nil {
		return err
	}
	return writeJSON(stdout, receipt)
}

func runShow(path string, args []string, stdout, stderr io.Writer) error {
	flags := newFlags("show", stderr)
	planID := flags.String("id", "", "plan or receipt identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("show does not accept positional arguments")
	}
	ledger, err := load(path)
	if err != nil {
		return err
	}
	if receipt, err := ledger.Receipt(*planID); err == nil {
		return writeJSON(stdout, receipt)
	}
	plan, ok := ledger.Plan(*planID)
	if !ok {
		return fmt.Errorf("plan or receipt not found: %s", *planID)
	}
	return writeJSON(stdout, plan)
}

func runSmoke(stdout, stderr io.Writer) error {
	directory, err := os.MkdirTemp("", "patchproof-smoke-")
	if err != nil {
		return fmt.Errorf("create smoke directory: %w", err)
	}
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, "ledger.json")
	steps := [][]string{
		{"--ledger", path, "open", "--id", "smoke-plan", "--note", "rehearsal"},
		{"--ledger", path, "connect", "--id", "smoke-plan", "--source", "stage-mic", "--destination", "desk-input"},
		{"--ledger", path, "connect", "--id", "smoke-plan", "--source", "desk-output", "--destination", "room-speaker"},
		{"--ledger", path, "activate", "--id", "smoke-plan"},
		{"--ledger", path, "close", "--id", "smoke-plan"},
	}
	for _, step := range steps {
		if code := run(step, io.Discard, stderr); code != 0 {
			return errors.New("smoke workflow step failed")
		}
	}
	var shown bytes.Buffer
	if code := run([]string{"--ledger", path, "show", "--id", "smoke-plan"}, &shown, stderr); code != 0 {
		return errors.New("smoke receipt lookup failed")
	}
	var receipt patchproof.Receipt
	if err := json.Unmarshal(shown.Bytes(), &receipt); err != nil {
		return fmt.Errorf("decode smoke receipt: %w", err)
	}
	if receipt.PlanID != "smoke-plan" || len(receipt.Connections) != 2 {
		return errors.New("smoke receipt was incomplete")
	}
	fmt.Fprintln(stdout, "smoke complete: 2 connections closed and receipt retrieved")
	return nil
}

// mutate loads the ledger, applies operation, and persists the result.
//
// Concurrent CLI invocations (possibly from different OS processes) can each
// load the same ledger state, apply their own change, and then save — the last
// save would silently overwrite the other's committed plan. To prevent that
// loss without serializing the potentially long-running operation itself,
// mutate uses optimistic concurrency control:
//
//  1. Load the ledger without holding the lock and apply operation. The
//     operation may block for arbitrary durations, so the lock must not be
//     held while it runs.
//  2. Acquire a cross-process file lock, reload the latest on-disk ledger, and
//     re-apply operation onto it. This merges the change onto the freshest
//     state instead of overwriting it. Conflicts surface as normal ledger
//     errors (duplicate plan, reserved endpoint, wrong state, etc.).
//  3. Save and release the lock.
func mutate(path string, operation func(*patchproof.Ledger) error) error {
	draft, err := load(path)
	if err != nil {
		return err
	}
	if err := operation(draft); err != nil {
		return err
	}

	release, err := lockLedger(path)
	if err != nil {
		return err
	}
	defer release()

	current, err := load(path)
	if err != nil {
		return err
	}
	if err := operation(current); err != nil {
		return err
	}
	return patchproof.Save(path, current)
}

// lockLedger acquires an exclusive advisory lock on a dedicated lock file next
// to the ledger. The returned function releases the lock and closes the lock
// handle. flock(2) locks are associated with the underlying file (inode), so
// using a separate, stable lock file keeps the lock valid even though Save
// atomically replaces the ledger file via rename.
func lockLedger(path string) (func(), error) {
	if path == "" {
		return nil, errors.New("ledger path cannot be blank")
	}
	lockPath := path + ".lock"
	file, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open ledger lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("lock ledger: %w", err)
	}
	released := false
	return func() {
		if released {
			return
		}
		released = true
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}

func load(path string) (*patchproof.Ledger, error) {
	ledger, err := patchproof.Load(path)
	if err == nil {
		return ledger, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return patchproof.NewLedger(), nil
	}
	return nil, err
}

func newFlags(name string, stderr io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	return flags
}

func writeJSON(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func reportError(stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, "patchproof:", err)
	return 1
}

func printUsage(output io.Writer) {
	fmt.Fprintln(output, "PatchProof coordinates temporary audio patch-bay changes.")
	fmt.Fprintln(output, "Usage: patchproof [--ledger path] <command> [flags]")
	fmt.Fprintln(output, "")
	fmt.Fprintln(output, "Commands:")
	fmt.Fprintln(output, "  open      create a draft plan")
	fmt.Fprintln(output, "  connect   add an ordered source-to-destination connection")
	fmt.Fprintln(output, "  activate  reserve every endpoint in a plan")
	fmt.Fprintln(output, "  close     release endpoints and create a receipt")
	fmt.Fprintln(output, "  show      display a plan or its receipt")
	fmt.Fprintln(output, "  smoke     run a complete bounded workflow in a temporary directory")
}
