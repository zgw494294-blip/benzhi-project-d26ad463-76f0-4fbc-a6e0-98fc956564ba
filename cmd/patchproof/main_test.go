package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"patchproof/patchproof"
)

func TestRunWorkflowPersistsAndShowsReceipt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.json")
	steps := [][]string{
		{"--ledger", path, "open", "--id", "showcase", "--note", "room A"},
		{"--ledger", path, "connect", "--id", "showcase", "--source", "vocal-mic", "--destination", "desk-1"},
		{"--ledger", path, "activate", "--id", "showcase"},
		{"--ledger", path, "close", "--id", "showcase"},
	}
	for _, step := range steps {
		if code := run(step, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
			t.Fatalf("command %v failed with code %d", step, code)
		}
	}
	var output bytes.Buffer
	if code := run([]string{"--ledger", path, "show", "--id", "showcase"}, &output, &bytes.Buffer{}); code != 0 {
		t.Fatal("show command failed")
	}
	var receipt patchproof.Receipt
	if err := json.Unmarshal(output.Bytes(), &receipt); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	if receipt.PlanID != "showcase" || receipt.Note != "room A" || len(receipt.Connections) != 1 {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
}

func TestRunWritesOperationalErrorsToStderr(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.json")
	var stderr bytes.Buffer
	if code := run([]string{"--ledger", path, "connect", "--id", "missing", "--source", "mic", "--destination", "desk"}, &bytes.Buffer{}, &stderr); code == 0 {
		t.Fatal("expected missing plan to fail")
	}
	if stderr.Len() == 0 {
		t.Fatal("expected operational error on stderr")
	}
}
