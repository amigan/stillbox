package incidents

import (
	"encoding/json"
	"strings"

	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/rbac/entities"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/google/uuid"
)

type Incident struct {
	ID          uuid.UUID          `json:"id"`
	Owner       users.UserID       `json:"owner"`
	Name        string             `json:"name"`
	Description *string            `json:"description,omitempty"`
	StartTime   *jsontypes.Time    `json:"startTime,omitempty"`
	EndTime     *jsontypes.Time    `json:"endTime,omitempty"`
	Location    jsontypes.Location `json:"location,omitempty"`
	Metadata    jsontypes.Metadata `json:"metadata,omitempty"`
	Calls       []IncidentCall     `json:"calls"`
}

func (inc *Incident) GetResourceName() string {
	return entities.ResourceIncident
}

func (inc *Incident) PlaylistFilename() string {
	rep := strings.NewReplacer(" ", "_", "/", "_", ":", "_")
	return rep.Replace(strings.ToLower(inc.Name))
}

type IncidentCall struct {
	calls.Call
	Notes json.RawMessage `json:"notes,omitempty"`
}
