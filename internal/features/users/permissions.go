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

// CanAssignInviteRole returns whether the inviter can assign the given role when creating an invitation.
// Only admins may assign admin or manager; managers may assign manager or member; both may assign member.
func CanAssignInviteRole(inviterRole, targetRole string) bool {
	switch targetRole {
	case RoleAdmin, RoleManager:
		return inviterRole == RoleAdmin
	case RoleMember:
		return inviterRole == RoleAdmin || inviterRole == RoleManager
	default:
		return false
	}
}
