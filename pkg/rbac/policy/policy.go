package policy

import (
	"dynatron.me/x/stillbox/pkg/rbac/entities"

	"github.com/el-mike/restrict/v2"
)

const (
	PresetUpdateOwn       = "updateOwn"
	PresetDeleteOwn       = "deleteOwn"
	PresetReadShared      = "readShared"
	PresetReadSharedInMap = "readSharedInMap"
	PresetShareOwn        = "shareOwn"

	PresetUpdateSubmitter      = "updateSubmitter"
	PresetDeleteSubmitter      = "deleteSubmitter"
	PresetShareSubmitter       = "shareSubmitter"
	PresetReadInSharedIncident = "readInSharedIncident"
	PresetReadPrivilegedSelf   = "readPrivileged"
	PresetUpdateSelf           = "updateSelf"
	PresetTranscribeCallID     = "transcribeCallID"
	PresetTranscribeSubmitter  = "transcribeSubmitter"
)

var Policy = &restrict.PolicyDefinition{
	Roles: restrict.Roles{
		entities.RoleUser: {
			Description: "An authenticated user",
			Grants: restrict.GrantsMap{
				entities.ResourceIncident: {
					&restrict.Permission{Action: entities.ActionRead},
					&restrict.Permission{Action: entities.ActionCreate},
					&restrict.Permission{Preset: PresetUpdateOwn},
					&restrict.Permission{Preset: PresetDeleteOwn},
					&restrict.Permission{Preset: PresetShareOwn},
				},
				entities.ResourceCall: {
					&restrict.Permission{Action: entities.ActionRead},
					&restrict.Permission{Action: entities.ActionCreate},
					&restrict.Permission{Preset: PresetUpdateSubmitter},
					&restrict.Permission{Preset: PresetDeleteSubmitter},
					&restrict.Permission{Preset: PresetShareSubmitter},
					&restrict.Permission{Preset: PresetTranscribeSubmitter},
				},
				entities.ResourceTalkgroup: {
					&restrict.Permission{Action: entities.ActionRead},
				},
				entities.ResourceShare: {
					&restrict.Permission{Action: entities.ActionRead},
					&restrict.Permission{Action: entities.ActionCreate},
					&restrict.Permission{Preset: PresetUpdateOwn},
					&restrict.Permission{Preset: PresetDeleteOwn},
				},
				entities.ResourceCallStats: {
					&restrict.Permission{Action: entities.ActionRead},
				},
				entities.ResourceSetting: {
					&restrict.Permission{Action: entities.ActionRead},
				},
				entities.ResourceUser: {
					&restrict.Permission{Action: entities.ActionRead},
					&restrict.Permission{Preset: PresetReadPrivilegedSelf},
					&restrict.Permission{Preset: PresetUpdateSelf},
				},
			},
		},
		entities.RoleSubmitter: {
			Description: "A role that can submit calls",
			Grants: restrict.GrantsMap{
				entities.ResourceCall: {
					&restrict.Permission{Action: entities.ActionCreate},
				},
				entities.ResourceTalkgroup: {
					// for learning TGs
					&restrict.Permission{Action: entities.ActionCreate},
					&restrict.Permission{Action: entities.ActionUpdate},
				},
			},
		},
		entities.RoleShareGuest: {
			Description: "Someone who has a valid share link",
			Grants: restrict.GrantsMap{
				entities.ResourceCall: {
					&restrict.Permission{Preset: PresetReadShared},
					&restrict.Permission{Preset: PresetReadInSharedIncident},
				},
				entities.ResourceIncident: {
					&restrict.Permission{Preset: PresetReadShared},
				},
				entities.ResourceTalkgroup: {
					&restrict.Permission{Action: entities.ActionRead},
				},
			},
		},
		entities.RoleAdmin: {
			Description: "A superuser",
			Parents:     []string{entities.RoleUser},
			Grants: restrict.GrantsMap{
				entities.ResourceIncident: {
					&restrict.Permission{Action: entities.ActionRead},
					&restrict.Permission{Action: entities.ActionUpdate},
					&restrict.Permission{Action: entities.ActionDelete},
					&restrict.Permission{Action: entities.ActionShare},
				},
				entities.ResourceCall: {
					&restrict.Permission{Action: entities.ActionRead},
					&restrict.Permission{Action: entities.ActionUpdate},
					&restrict.Permission{Action: entities.ActionDelete},
					&restrict.Permission{Action: entities.ActionShare},
					&restrict.Permission{Action: entities.ActionTranscribe},
				},
				entities.ResourceTalkgroup: {
					&restrict.Permission{Action: entities.ActionRead},
					&restrict.Permission{Action: entities.ActionUpdate},
					&restrict.Permission{Action: entities.ActionCreate},
					&restrict.Permission{Action: entities.ActionDelete},
				},
				entities.ResourceShare: {
					&restrict.Permission{Action: entities.ActionRead},
					&restrict.Permission{Action: entities.ActionUpdate},
					&restrict.Permission{Action: entities.ActionCreate},
					&restrict.Permission{Action: entities.ActionDelete},
				},
				entities.ResourceSetting: {
					&restrict.Permission{Action: entities.ActionRead},
					&restrict.Permission{Action: entities.ActionCreate},
					&restrict.Permission{Action: entities.ActionUpdate},
					&restrict.Permission{Action: entities.ActionDelete},
				},
				entities.ResourceUser: {
					&restrict.Permission{Action: entities.ActionReadPrivileged},
					&restrict.Permission{Action: entities.ActionUpdatePrivileged},
					&restrict.Permission{Action: entities.ActionCreate},
					&restrict.Permission{Action: entities.ActionUpdate},
					&restrict.Permission{Action: entities.ActionDelete},
				},
			},
		},
		entities.RoleSystem: {
			Description: "A system service",
			Parents:     []string{entities.RoleAdmin},
		},
		entities.RolePublic: {
			Description: "Everybody else",
			Grants: restrict.GrantsMap{
				entities.ResourceShare: {
					&restrict.Permission{Action: entities.ActionRead},
				},
			},
		},
		entities.RoleTranscriber: {
			Description: "Call transcription service",
			Grants: restrict.GrantsMap{
				entities.ResourceCall: {
					&restrict.Permission{Preset: PresetTranscribeCallID},
				},
			},
		},
	},
	PermissionPresets: restrict.PermissionPresets{
		PresetUpdateOwn: &restrict.Permission{
			Action: entities.ActionUpdate,
			Conditions: restrict.Conditions{
				&restrict.EqualCondition{
					ID: "isOwner",
					Left: &restrict.ValueDescriptor{
						Source: restrict.ResourceField,
						Field:  "Owner",
					},
					Right: &restrict.ValueDescriptor{
						Source: restrict.SubjectField,
						Field:  "ID",
					},
				},
			},
		},
		PresetDeleteOwn: &restrict.Permission{
			Action: entities.ActionDelete,
			Conditions: restrict.Conditions{
				&restrict.EqualCondition{
					ID: "isOwner",
					Left: &restrict.ValueDescriptor{
						Source: restrict.ResourceField,
						Field:  "Owner",
					},
					Right: &restrict.ValueDescriptor{
						Source: restrict.SubjectField,
						Field:  "ID",
					},
				},
			},
		},
		PresetShareOwn: &restrict.Permission{
			Action: entities.ActionShare,
			Conditions: restrict.Conditions{
				&restrict.EqualCondition{
					ID: "isOwner",
					Left: &restrict.ValueDescriptor{
						Source: restrict.ResourceField,
						Field:  "Owner",
					},
					Right: &restrict.ValueDescriptor{
						Source: restrict.SubjectField,
						Field:  "ID",
					},
				},
			},
		},
		PresetUpdateSubmitter: &restrict.Permission{
			Action: entities.ActionUpdate,
			Conditions: restrict.Conditions{
				&SubmitterEqualCondition{
					ID: "isSubmitter",
					Left: &restrict.ValueDescriptor{
						Source: restrict.ResourceField,
						Field:  "Submitter",
					},
					Right: &restrict.ValueDescriptor{
						Source: restrict.SubjectField,
						Field:  "ID",
					},
				},
			},
		},
		PresetDeleteSubmitter: &restrict.Permission{
			Action: entities.ActionDelete,
			Conditions: restrict.Conditions{
				&SubmitterEqualCondition{
					ID: "isSubmitter",
					Left: &restrict.ValueDescriptor{
						Source: restrict.ResourceField,
						Field:  "Submitter",
					},
					Right: &restrict.ValueDescriptor{
						Source: restrict.SubjectField,
						Field:  "ID",
					},
				},
			},
		},
		PresetShareSubmitter: &restrict.Permission{
			Action: entities.ActionShare,
			Conditions: restrict.Conditions{
				&SubmitterEqualCondition{
					ID: "isSubmitter",
					Left: &restrict.ValueDescriptor{
						Source: restrict.ResourceField,
						Field:  "Submitter",
					},
					Right: &restrict.ValueDescriptor{
						Source: restrict.SubjectField,
						Field:  "ID",
					},
				},
			},
		},
		PresetReadShared: &restrict.Permission{
			Action: entities.ActionRead,
			Conditions: restrict.Conditions{
				&restrict.EqualCondition{
					ID: "isOwner",
					Left: &restrict.ValueDescriptor{
						Source: restrict.ResourceField,
						Field:  "ID",
					},
					Right: &restrict.ValueDescriptor{
						Source: restrict.SubjectField,
						Field:  "EntityID",
					},
				},
			},
		},
		PresetReadInSharedIncident: &restrict.Permission{
			Action: entities.ActionRead,
			Conditions: restrict.Conditions{
				&CallInIncidentCondition{
					ID: "callInIncident",
					Call: &restrict.ValueDescriptor{
						Source: restrict.ResourceField,
						Field:  "ID",
					},
					Incident: &restrict.ValueDescriptor{
						Source: restrict.SubjectField,
						Field:  "EntityID",
					},
				},
			},
		},
		PresetReadPrivilegedSelf: &restrict.Permission{
			Action: entities.ActionReadPrivileged,
			Conditions: restrict.Conditions{
				&restrict.EqualCondition{
					ID: "userSelf",
					Left: &restrict.ValueDescriptor{
						Source: restrict.SubjectField,
						Field:  "ID",
					},
					Right: &restrict.ValueDescriptor{
						Source: restrict.ResourceField,
						Field:  "ID",
					},
				},
			},
		},
		PresetUpdateSelf: &restrict.Permission{
			Action: entities.ActionUpdate,
			Conditions: restrict.Conditions{
				&restrict.EqualCondition{
					ID: "userSelf",
					Left: &restrict.ValueDescriptor{
						Source: restrict.SubjectField,
						Field:  "ID",
					},
					Right: &restrict.ValueDescriptor{
						Source: restrict.ResourceField,
						Field:  "ID",
					},
				},
			},
		},
		PresetTranscribeCallID: &restrict.Permission{
			Action: entities.ActionTranscribe,
			Conditions: restrict.Conditions{
				&restrict.EqualCondition{
					ID: "callIDSame",
					Left: &restrict.ValueDescriptor{
						Source: restrict.SubjectField,
						Field:  "CallID",
					},
					Right: &restrict.ValueDescriptor{
						Source: restrict.ResourceField,
						Field:  "ID",
					},
				},
			},
		},
		PresetTranscribeSubmitter: &restrict.Permission{
			Action: entities.ActionTranscribe,
			Conditions: restrict.Conditions{
				&SubmitterEqualCondition{
					ID: "isSubmitter",
					Left: &restrict.ValueDescriptor{
						Source: restrict.ResourceField,
						Field:  "Submitter",
					},
					Right: &restrict.ValueDescriptor{
						Source: restrict.SubjectField,
						Field:  "ID",
					},
				},
			},
		},
	},
}
