package rbac

const (
	RoleUser       = "User"
	RoleSubmitter  = "Submitter"
	RoleAdmin      = "Admin"
	RoleSystem     = "System"
	RolePublic     = "Public"
	RoleShareGuest = "ShareGuest"

	ResourceCall      = "Call"
	ResourceIncident  = "Incident"
	ResourceTalkgroup = "Talkgroup"
	ResourceAlert     = "Alert"
	ResourceShare     = "Share"
	ResourceAPIKey    = "APIKey"

	ActionRead   = "read"
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionShare  = "share"
)
