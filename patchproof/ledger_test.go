package patchproof

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesDraftWithNormalizedNote(t *testing.T) {
	ledger := NewLedger()

	plan, err := ledger.Open("room-a", "")
	if err != nil {
		t.Fatalf("open plan: %v", err)
	}
	if plan.ID != "room-a" || plan.Note != "" || plan.Status != StatusDraft {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if _, err := ledger.Open(" ", "ignored"); err == nil {
		t.Fatal("expected blank identifier to be rejected")
	}
	if _, err := ledger.Open("room-a", "duplicate"); err == nil {
		t.Fatal("expected duplicate identifier to be rejected")
	}
}

func TestConnectValidatesEndpointsAndProtectsStoredConnections(t *testing.T) {
	ledger := NewLedger()
	if _, err := ledger.Open("plan", "operator"); err != nil {
		t.Fatalf("open plan: %v", err)
	}
	for _, pair := range [][2]string{{"", "out"}, {"in", " "}, {"in", "in"}} {
		if _, err := ledger.Connect("plan", pair[0], pair[1]); err == nil {
			t.Fatalf("expected invalid connection %q -> %q to fail", pair[0], pair[1])
		}
	}
	if _, err := ledger.Connect("plan", "mic", "console"); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := ledger.Connect("plan", "console", "recorder"); err == nil {
		t.Fatal("expected endpoint reuse to fail")
	}
	plan, ok := ledger.Plan("plan")
	if !ok {
		t.Fatal("expected plan")
	}
	plan.Connections[0].Source = "changed"
	stored, _ := ledger.Plan("plan")
	if stored.Connections[0].Source != "mic" {
		t.Fatalf("stored connection was mutated through return value: %+v", stored.Connections)
	}
}

func TestActivateConflictLeavesAllPlansUnchanged(t *testing.T) {
	ledger := NewLedger()
	for _, id := range []string{"occupied", "conflict", "other"} {
		if _, err := ledger.Open(id, ""); err != nil {
			t.Fatalf("open %s: %v", id, err)
		}
	}
	if _, err := ledger.Connect("occupied", "mic", "console"); err != nil {
		t.Fatalf("connect occupied: %v", err)
	}
	if err := ledger.Activate("occupied"); err != nil {
		t.Fatalf("activate occupied: %v", err)
	}
	if _, err := ledger.Connect("conflict", "spare", "mic"); err != nil {
		t.Fatalf("connect conflict: %v", err)
	}
	if err := ledger.Activate("conflict"); err == nil {
		t.Fatal("expected activation conflict")
	}
	conflict, _ := ledger.Plan("conflict")
	if conflict.Status != StatusDraft {
		t.Fatalf("conflicting plan changed state: %+v", conflict)
	}
	if _, err := ledger.Connect("other", "spare", "amp"); err != nil {
		t.Fatalf("connect other: %v", err)
	}
	if err := ledger.Activate("other"); err != nil {
		t.Fatalf("spare endpoint was reserved by failed activation: %v", err)
	}
}

func TestCloseCreatesReceiptReleasesReservationsAndCopiesReceipt(t *testing.T) {
	ledger := NewLedger()
	if _, err := ledger.Open("plan", "note"); err != nil {
		t.Fatalf("open plan: %v", err)
	}
	if _, err := ledger.Connect("plan", "mic", "console"); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := ledger.Activate("plan"); err != nil {
		t.Fatalf("activate: %v", err)
	}
	receipt, err := ledger.Close("plan")
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	receipt.Connections[0].Destination = "changed"
	stored, err := ledger.Receipt("plan")
	if err != nil {
		t.Fatalf("get receipt: %v", err)
	}
	if stored.Connections[0].Destination != "console" {
		t.Fatalf("stored receipt was mutated: %+v", stored.Connections)
	}
	closed, _ := ledger.Plan("plan")
	if closed.Status != StatusClosed {
		t.Fatalf("plan was not closed: %+v", closed)
	}
	if _, err := ledger.Close("plan"); err == nil {
		t.Fatal("expected repeated close to fail")
	}
	if _, err := ledger.Open("replacement", ""); err != nil {
		t.Fatalf("open replacement: %v", err)
	}
	if _, err := ledger.Connect("replacement", "mic", "amp"); err != nil {
		t.Fatalf("connect replacement: %v", err)
	}
	if err := ledger.Activate("replacement"); err != nil {
		t.Fatalf("closed plan did not release reservations: %v", err)
	}
}

func TestSaveAndLoadRoundTripUsesTheCompleteLedger(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "ledger.json")
	ledger := NewLedger()
	if _, err := ledger.Open("plan", "operator"); err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := ledger.Connect("plan", "mic", "console"); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := ledger.Activate("plan"); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if err := Save(path, ledger); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	plan, ok := loaded.Plan("plan")
	if !ok || plan.Status != StatusActive || len(plan.Connections) != 1 {
		t.Fatalf("round-trip lost plan: %+v", plan)
	}
	if _, err := loaded.Close("plan"); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := Save(path, loaded); err != nil {
		t.Fatalf("save closed ledger: %v", err)
	}
	loaded, err = Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if _, err := loaded.Receipt("plan"); err != nil {
		t.Fatalf("round-trip lost receipt: %v", err)
	}
}

func TestFailedSaveCleansTemporaryArtifactAndLoadErrorsPropagate(t *testing.T) {
	directory := t.TempDir()
	badTarget := filepath.Join(directory, "ledger.json")
	if err := os.Mkdir(badTarget, 0o700); err != nil {
		t.Fatalf("make invalid target: %v", err)
	}
	if err := Save(badTarget, NewLedger()); err == nil {
		t.Fatal("expected save to fail when target is a directory")
	}
	artifacts, err := filepath.Glob(filepath.Join(directory, ".ledger.json.tmp-*"))
	if err != nil {
		t.Fatalf("find temporary artifacts: %v", err)
	}
	if len(artifacts) != 0 {
		t.Fatalf("failed save left temporary artifacts: %v", artifacts)
	}
	invalid := filepath.Join(directory, "invalid.json")
	if err := os.WriteFile(invalid, []byte("not-json"), 0o600); err != nil {
		t.Fatalf("write invalid ledger: %v", err)
	}
	if _, err := Load(invalid); err == nil {
		t.Fatal("expected invalid ledger load to fail")
	}
}
