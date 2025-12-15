package incidents

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/google/uuid"
)

type Incident struct {
	ID          uuid.UUID          `json:"id"`
	OwnerID     users.UserID       `json:"-"`
	Owner       string             `json:"owner"`
	Name        string             `json:"name"`
	Description *string            `json:"description,omitempty"`
	CreatedAt   *jsontypes.Time    `json:"createdAt"`
	StartTime   *jsontypes.Time    `json:"startTime,omitempty"`
	EndTime     *jsontypes.Time    `json:"endTime,omitempty"`
	Location    jsontypes.Location `json:"location"`
	Metadata    jsontypes.Metadata `json:"metadata,omitempty"`
	Calls       []IncidentCall     `json:"calls,omitempty"`
}

type IncidentCalls struct {
	Calls []IncidentCall `json:"calls"`
	Count int            `json:"count"`
}

func (inc *Incident) SetShareURL(bu url.URL, shareID string) {
	bu.Path = fmt.Sprintf("/share/%s/call/", shareID)
	for i := range inc.Calls {
		if inc.Calls[i].AudioURL != nil {
			continue
		}
		inc.Calls[i].AudioURL = common.PtrTo(bu.String() + inc.Calls[i].ID.String())
	}
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
