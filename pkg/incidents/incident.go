package incidents

import (
	"encoding/json"

	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/calls"
	"github.com/google/uuid"
)

type Incident struct {
	ID          uuid.UUID          `json:"id"`
	Name        string             `json:"name"`
	Description *string            `json:"description"`
	StartTime   *jsontypes.Time    `json:"startTime"`
	EndTime     *jsontypes.Time    `json:"endTime"`
	Location    jsontypes.Location `json:"location"`
	Metadata    jsontypes.Metadata `json:"metadata"`
	Calls       []IncidentCall     `json:"calls"`
}

type IncidentCall struct {
	calls.Call
	Notes json.RawMessage `json:"notes"`
}
