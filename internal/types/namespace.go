package types

// A Namespace is a user or organization. Exactly 1 of UserID and OrgID must be set.
type Namespace struct {
	User *int32 // user ID
	Org  *int32 // organization ID
}

func (n Namespace) Equals(other Namespace) bool {
	if n.User != nil && other.User != nil {
		return *n.User == *other.User
	}
	if n.Org != nil && other.Org != nil {
		return *n.Org == *other.Org
	}
	return false
}

func NamespaceUser(userID int32) Namespace {
	return Namespace{User: &userID}
}

func NamespaceOrg(orgID int32) Namespace {
	return Namespace{Org: &orgID}
}
