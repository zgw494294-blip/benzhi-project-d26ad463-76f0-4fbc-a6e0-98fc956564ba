package patchproof

// Status describes the point a patch plan has reached in its lifecycle.
type Status string

const (
	StatusDraft  Status = "draft"
	StatusActive Status = "active"
	StatusClosed Status = "closed"
)

// Connection is one ordered source-to-destination patch.
type Connection struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// Plan is a patch plan and its current lifecycle state.
type Plan struct {
	ID          string       `json:"id"`
	Note        string       `json:"note"`
	Status      Status       `json:"status"`
	Connections []Connection `json:"connections"`
}

// Receipt is the closed, read-only record of a completed patch plan.
type Receipt struct {
	PlanID      string       `json:"plan_id"`
	Note        string       `json:"note"`
	Connections []Connection `json:"connections"`
}

func cloneConnections(connections []Connection) []Connection {
	if len(connections) == 0 {
		return []Connection{}
	}
	copyOf := make([]Connection, len(connections))
	copy(copyOf, connections)
	return copyOf
}

func clonePlan(plan Plan) Plan {
	plan.Connections = cloneConnections(plan.Connections)
	return plan
}

func cloneReceipt(receipt Receipt) Receipt {
	receipt.Connections = cloneConnections(receipt.Connections)
	return receipt
}
