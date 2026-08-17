package patchproof

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrPlanNotFound      = errors.New("plan not found")
	ErrReceiptNotFound   = errors.New("receipt not found")
	ErrPlanState         = errors.New("plan is not in the required state")
	ErrEndpointReserved  = errors.New("endpoint is already reserved")
	ErrDuplicateEndpoint = errors.New("endpoint is already used by this plan")
)

const ledgerVersion = 1

// Ledger holds patch plans, receipts, and active endpoint ownership.
// The maps are private so every returned collection can be copied safely.
type Ledger struct {
	version  int
	plans    map[string]Plan
	receipts map[string]Receipt
	occupied map[string]string
}

// NewLedger returns an empty versioned ledger.
func NewLedger() *Ledger {
	return &Ledger{
		version:  ledgerVersion,
		plans:    make(map[string]Plan),
		receipts: make(map[string]Receipt),
		occupied: make(map[string]string),
	}
}

func (l *Ledger) ensureMaps() {
	if l.plans == nil {
		l.plans = make(map[string]Plan)
	}
	if l.receipts == nil {
		l.receipts = make(map[string]Receipt)
	}
	if l.occupied == nil {
		l.occupied = make(map[string]string)
	}
}

// Open creates a uniquely identified draft. Omitted and empty notes both become
// the same empty value.
func (l *Ledger) Open(id, note string) (Plan, error) {
	l.ensureMaps()
	id = normalize(id)
	if id == "" {
		return Plan{}, errors.New("plan identifier cannot be blank")
	}
	if _, exists := l.plans[id]; exists {
		return Plan{}, fmt.Errorf("%w: %s", ErrPlanState, id)
	}
	plan := Plan{ID: id, Note: normalize(note), Status: StatusDraft, Connections: []Connection{}}
	l.plans[id] = plan
	return clonePlan(plan), nil
}

// Connect appends one connection while the plan is still a draft.
func (l *Ledger) Connect(planID, source, destination string) (Connection, error) {
	l.ensureMaps()
	planID = normalize(planID)
	plan, ok := l.plans[planID]
	if !ok {
		return Connection{}, fmt.Errorf("%w: %s", ErrPlanNotFound, planID)
	}
	if plan.Status != StatusDraft {
		return Connection{}, fmt.Errorf("%w: connect requires draft plan %s", ErrPlanState, planID)
	}
	connection, err := newConnection(source, destination)
	if err != nil {
		return Connection{}, err
	}
	for _, existing := range plan.Connections {
		if existing.Source == connection.Source || existing.Destination == connection.Source ||
			existing.Source == connection.Destination || existing.Destination == connection.Destination {
			return Connection{}, fmt.Errorf("%w: %s", ErrDuplicateEndpoint, connection.Source)
		}
	}
	plan.Connections = append(plan.Connections, connection)
	l.plans[planID] = plan
	return connection, nil
}

// Activate reserves every endpoint in one all-or-nothing state transition.
func (l *Ledger) Activate(planID string) error {
	l.ensureMaps()
	planID = normalize(planID)
	plan, ok := l.plans[planID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrPlanNotFound, planID)
	}
	if plan.Status != StatusDraft {
		return fmt.Errorf("%w: activate requires draft plan %s", ErrPlanState, planID)
	}
	if len(plan.Connections) == 0 {
		return errors.New("cannot activate an empty plan")
	}
	for _, connection := range plan.Connections {
		for _, endpoint := range []string{connection.Source, connection.Destination} {
			if owner, reserved := l.occupied[endpoint]; reserved && owner != planID {
				return fmt.Errorf("%w: %s", ErrEndpointReserved, endpoint)
			}
		}
		l.occupied[connection.Source] = planID
		l.occupied[connection.Destination] = planID
	}
	plan.Status = StatusActive
	l.plans[planID] = plan
	return nil
}

// Close releases this plan's reservations and records its final connections.
func (l *Ledger) Close(planID string) (Receipt, error) {
	l.ensureMaps()
	planID = normalize(planID)
	plan, ok := l.plans[planID]
	if !ok {
		return Receipt{}, fmt.Errorf("%w: %s", ErrPlanNotFound, planID)
	}
	if plan.Status != StatusActive {
		return Receipt{}, fmt.Errorf("%w: close requires active plan %s", ErrPlanState, planID)
	}
	for _, connection := range plan.Connections {
		for _, endpoint := range []string{connection.Source, connection.Destination} {
			if owner, reserved := l.occupied[endpoint]; reserved && owner == planID {
				delete(l.occupied, endpoint)
			}
		}
	}
	receipt := Receipt{PlanID: plan.ID, Note: plan.Note, Connections: cloneConnections(plan.Connections)}
	plan.Status = StatusClosed
	l.plans[planID] = plan
	l.receipts[planID] = receipt
	return cloneReceipt(receipt), nil
}

// Plan returns a defensive copy of a plan.
func (l *Ledger) Plan(id string) (Plan, bool) {
	plan, ok := l.plans[normalize(id)]
	if !ok {
		return Plan{}, false
	}
	return clonePlan(plan), true
}

// Receipt returns a defensive copy of a closed receipt.
func (l *Ledger) Receipt(id string) (Receipt, error) {
	receipt, ok := l.receipts[normalize(id)]
	if !ok {
		return Receipt{}, fmt.Errorf("%w: %s", ErrReceiptNotFound, normalize(id))
	}
	return cloneReceipt(receipt), nil
}

func newConnection(source, destination string) (Connection, error) {
	source = normalize(source)
	destination = normalize(destination)
	if source == "" || destination == "" {
		return Connection{}, errors.New("connection endpoints cannot be blank")
	}
	if source == destination {
		return Connection{}, errors.New("connection cannot target itself")
	}
	return Connection{Source: source, Destination: destination}, nil
}

func normalize(value string) string {
	return strings.TrimSpace(value)
}
