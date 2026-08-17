package patchproof

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type ledgerFile struct {
	Version  int                `json:"version"`
	Plans    map[string]Plan    `json:"plans"`
	Receipts map[string]Receipt `json:"receipts"`
	Occupied map[string]string  `json:"occupied"`
}

// Load reads and validates one complete versioned ledger.
func Load(path string) (*Ledger, error) {
	if path == "" {
		return nil, errors.New("ledger path cannot be blank")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load ledger: %w", err)
	}
	var stored ledgerFile
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("decode ledger: %w", err)
	}
	ledger := &Ledger{
		version:  stored.Version,
		plans:    stored.Plans,
		receipts: stored.Receipts,
		occupied: stored.Occupied,
	}
	ledger.ensureMaps()
	if err := ledger.validate(); err != nil {
		return nil, fmt.Errorf("validate ledger: %w", err)
	}
	return ledger, nil
}

// Save atomically replaces path with the complete ledger.
func Save(path string, ledger *Ledger) error {
	if path == "" {
		return errors.New("ledger path cannot be blank")
	}
	if ledger == nil {
		return errors.New("ledger cannot be nil")
	}
	if err := ledger.validate(); err != nil {
		return fmt.Errorf("validate ledger before save: %w", err)
	}
	data, err := json.MarshalIndent(ledger.fileValue(), "", "  ")
	if err != nil {
		return fmt.Errorf("encode ledger: %w", err)
	}
	directory := filepath.Dir(path)
	base := filepath.Base(path)
	temporary, err := os.CreateTemp(directory, "."+base+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary ledger: %w", err)
	}
	temporaryName := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryName)
		}
	}()

	if written, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary ledger: %w", err)
	} else if written != len(data) {
		_ = temporary.Close()
		return fmt.Errorf("write temporary ledger: %w", io.ErrShortWrite)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary ledger: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary ledger: %w", err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("replace ledger: %w", err)
	}
	removeTemporary = false
	return nil
}

func (l *Ledger) fileValue() ledgerFile {
	l.ensureMaps()
	plans := make(map[string]Plan, len(l.plans))
	for id, plan := range l.plans {
		plans[id] = clonePlan(plan)
	}
	receipts := make(map[string]Receipt, len(l.receipts))
	for id, receipt := range l.receipts {
		receipts[id] = cloneReceipt(receipt)
	}
	occupied := make(map[string]string, len(l.occupied))
	for endpoint, owner := range l.occupied {
		occupied[endpoint] = owner
	}
	return ledgerFile{Version: l.version, Plans: plans, Receipts: receipts, Occupied: occupied}
}

func (l *Ledger) validate() error {
	if l.version != ledgerVersion {
		return fmt.Errorf("unsupported ledger version %d", l.version)
	}
	l.ensureMaps()
	for id, plan := range l.plans {
		if id == "" || plan.ID != id || normalize(id) != id {
			return fmt.Errorf("invalid plan identifier %q", id)
		}
		if plan.Note != normalize(plan.Note) {
			return fmt.Errorf("plan %q has an unnormalized note", id)
		}
		switch plan.Status {
		case StatusDraft, StatusActive, StatusClosed:
		default:
			return fmt.Errorf("plan %q has invalid status %q", id, plan.Status)
		}
		if plan.Status == StatusClosed {
			if _, exists := l.receipts[id]; !exists {
				return fmt.Errorf("closed plan %q is missing its receipt", id)
			}
		}
		seen := make(map[string]struct{}, len(plan.Connections)*2)
		for _, connection := range plan.Connections {
			canonical, err := newConnection(connection.Source, connection.Destination)
			if err != nil || canonical != connection {
				return fmt.Errorf("plan %q has invalid connection", id)
			}
			for _, endpoint := range []string{connection.Source, connection.Destination} {
				if _, exists := seen[endpoint]; exists {
					return fmt.Errorf("plan %q reuses endpoint %q", id, endpoint)
				}
				seen[endpoint] = struct{}{}
			}
		}
		if plan.Status != StatusActive {
			for endpoint := range seen {
				if owner, reserved := l.occupied[endpoint]; reserved && owner == id {
					return fmt.Errorf("non-active plan %q owns endpoint %q", id, endpoint)
				}
			}
		} else {
			for endpoint := range seen {
				if owner, reserved := l.occupied[endpoint]; !reserved || owner != id {
					return fmt.Errorf("active plan %q is missing endpoint %q", id, endpoint)
				}
			}
		}
	}
	for id, receipt := range l.receipts {
		plan, exists := l.plans[id]
		if !exists || plan.Status != StatusClosed || receipt.PlanID != id || receipt.Note != plan.Note ||
			!sameConnections(receipt.Connections, plan.Connections) {
			return fmt.Errorf("receipt %q does not match its closed plan", id)
		}
	}
	for endpoint, owner := range l.occupied {
		if endpoint == "" {
			return errors.New("occupied endpoint cannot be blank")
		}
		plan, exists := l.plans[owner]
		if !exists || plan.Status != StatusActive || !planUsesEndpoint(plan, endpoint) {
			return fmt.Errorf("occupied endpoint %q has invalid owner %q", endpoint, owner)
		}
	}
	return nil
}

func planUsesEndpoint(plan Plan, endpoint string) bool {
	for _, connection := range plan.Connections {
		if connection.Source == endpoint || connection.Destination == endpoint {
			return true
		}
	}
	return false
}

func sameConnections(left, right []Connection) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
