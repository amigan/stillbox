package rbac

import (
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
)

var Policy = &restrict.PolicyDefinition{
	Roles: restrict.Roles{
		RoleUser: {
			Description: "An authenticated user",
			Grants: restrict.GrantsMap{
				ResourceIncident: {
					&restrict.Permission{Action: ActionRead},
					&restrict.Permission{Action: ActionCreate},
					&restrict.Permission{Preset: PresetUpdateOwn},
					&restrict.Permission{Preset: PresetDeleteOwn},
					&restrict.Permission{Preset: PresetShareOwn},
				},
				ResourceCall: {
					&restrict.Permission{Action: ActionRead},
					&restrict.Permission{Action: ActionCreate},
					&restrict.Permission{Preset: PresetUpdateSubmitter},
					&restrict.Permission{Preset: PresetDeleteSubmitter},
					&restrict.Permission{Action: ActionShare},
				},
				ResourceTalkgroup: {
					&restrict.Permission{Action: ActionRead},
				},
				ResourceShare: {
					&restrict.Permission{Action: ActionRead},
					&restrict.Permission{Action: ActionCreate},
					&restrict.Permission{Preset: PresetUpdateOwn},
					&restrict.Permission{Preset: PresetDeleteOwn},
				},
			},
		},
		RoleSubmitter: {
			Description: "A role that can submit calls",
			Grants: restrict.GrantsMap{
				ResourceCall: {
					&restrict.Permission{Action: ActionCreate},
				},
				ResourceTalkgroup: {
					// for learning TGs
					&restrict.Permission{Action: ActionCreate},
					&restrict.Permission{Action: ActionUpdate},
				},
			},
		},
		RoleShareGuest: {
			Description: "Someone who has a valid share link",
			Grants: restrict.GrantsMap{
				ResourceCall: {
					&restrict.Permission{Preset: PresetReadShared},
					&restrict.Permission{Preset: PresetReadInSharedIncident},
				},
				ResourceIncident: {
					&restrict.Permission{Preset: PresetReadShared},
				},
				ResourceTalkgroup: {
					&restrict.Permission{Action: ActionRead},
				},
			},
		},
		RoleAdmin: {
			Parents: []string{RoleUser},
			Grants: restrict.GrantsMap{
				ResourceIncident: {
					&restrict.Permission{Action: ActionUpdate},
					&restrict.Permission{Action: ActionDelete},
					&restrict.Permission{Action: ActionShare},
				},
				ResourceCall: {
					&restrict.Permission{Action: ActionUpdate},
					&restrict.Permission{Action: ActionDelete},
					&restrict.Permission{Action: ActionShare},
				},
				ResourceTalkgroup: {
					&restrict.Permission{Action: ActionUpdate},
					&restrict.Permission{Action: ActionCreate},
					&restrict.Permission{Action: ActionDelete},
				},
			},
		},
		RoleSystem: {
			Parents: []string{RoleSystem},
		},
		RolePublic: {
			/*
				Grants: restrict.GrantsMap{
					ResourceShare: {
						&restrict.Permission{Action: ActionRead},
					},
				},
			*/
		},
	},
	PermissionPresets: restrict.PermissionPresets{
		PresetUpdateOwn: &restrict.Permission{
			Action: ActionUpdate,
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
			Action: ActionDelete,
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
			Action: ActionShare,
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
			Action: ActionUpdate,
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
			Action: ActionDelete,
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
			Action: ActionShare,
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
			Action: ActionRead,
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
			Action: ActionRead,
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
	},
}
