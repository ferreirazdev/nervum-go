package user

func CanViewOrganization(role string) bool {
	return role == RoleAdmin || role == RoleManager
}

func CanEditOrganization(role string) bool {
	return role == RoleAdmin
}

func CanManageTeams(role string) bool {
	return role == RoleAdmin || role == RoleManager
}

func CanViewAllTeams(role string) bool {
	return role == RoleAdmin || role == RoleManager
}

func CanManageEnvironments(role string) bool {
	return role == RoleAdmin || role == RoleManager
}

func CanViewAllEnvironments(role string) bool {
	return role == RoleAdmin || role == RoleManager
}

func CanInvite(role string) bool {
	return role == RoleAdmin || role == RoleManager
}

func CanListOrgMembers(role string) bool {
	return role == RoleAdmin || role == RoleManager
}
