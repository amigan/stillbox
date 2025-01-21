package incidents

import (
	"encoding/json"

	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/rbac"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/google/uuid"
)

type Incident struct {
	ID          uuid.UUID          `json:"id"`
	Owner       users.UserID       `json:"owner"`
	Name        string             `json:"name"`
	Description *string            `json:"description"`
	StartTime   *jsontypes.Time    `json:"startTime"`
	EndTime     *jsontypes.Time    `json:"endTime"`
	Location    jsontypes.Location `json:"location"`
	Metadata    jsontypes.Metadata `json:"metadata"`
	Calls       []IncidentCall     `json:"calls"`
}

func (inc *Incident) GetResourceName() string {
	return rbac.ResourceIncident
}

type IncidentCall struct {
	calls.Call
	Notes json.RawMessage `json:"notes"`
}
