package testutil

type AdminSubject struct{}

func (*AdminSubject) String() string     { return "test" }
func (*AdminSubject) GetName() string    { return "test" }
func (*AdminSubject) GetRoles() []string { return []string{"admin"} }
