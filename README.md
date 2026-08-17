# PatchProof

PatchProof is a small standard-library Go CLI for coordinating temporary audio patch-bay changes. A plan moves through `draft`, `active`, and `closed` states. Activating a plan reserves every endpoint; closing it releases those reservations and records a receipt.

## Requirements

- Go 1.22 or newer
- No external services

## Commands

The ledger defaults to `.patchproof.json` in the current directory. Use `--ledger` before the command to choose another path.

```text
go run ./cmd/patchproof --ledger rehearsal.json open --id vocal-change --note "Room A"
go run ./cmd/patchproof --ledger rehearsal.json connect --id vocal-change --source vocal-mic --destination desk-input
go run ./cmd/patchproof --ledger rehearsal.json connect --id vocal-change --source desk-output --destination room-speaker
go run ./cmd/patchproof --ledger rehearsal.json activate --id vocal-change
go run ./cmd/patchproof --ledger rehearsal.json close --id vocal-change
go run ./cmd/patchproof --ledger rehearsal.json show --id vocal-change
```

Each successful command emits JSON. Plans reject blank or reused endpoints, and activation fails without changing the ledger when another active plan owns an endpoint. Connections are ordered, and a closed receipt cannot be changed through values returned by the Go package.

## Verification

```text
go test ./...
go run ./cmd/patchproof smoke
```

The smoke command runs the complete open, connect, activate, close, and receipt lookup flow in a temporary directory and removes it before exiting.
