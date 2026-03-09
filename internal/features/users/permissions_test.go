package user

import (
	"testing"
)

func TestCanViewOrganization_Unit(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{RoleAdmin, true},
		{RoleManager, true},
		{RoleMember, false},
		{"", false},
		{"invalid", false},
	}
	for _, tt := range tests {
		if got := CanViewOrganization(tt.role); got != tt.want {
			t.Errorf("CanViewOrganization(%q) = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestCanEditOrganization_Unit(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{RoleAdmin, true},
		{RoleManager, false},
		{RoleMember, false},
		{"", false},
	}
	for _, tt := range tests {
		if got := CanEditOrganization(tt.role); got != tt.want {
			t.Errorf("CanEditOrganization(%q) = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestCanManageTeams_Unit(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{RoleAdmin, true},
		{RoleManager, true},
		{RoleMember, false},
	}
	for _, tt := range tests {
		if got := CanManageTeams(tt.role); got != tt.want {
			t.Errorf("CanManageTeams(%q) = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestCanViewAllTeams_Unit(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{RoleAdmin, true},
		{RoleManager, true},
		{RoleMember, false},
	}
	for _, tt := range tests {
		if got := CanViewAllTeams(tt.role); got != tt.want {
			t.Errorf("CanViewAllTeams(%q) = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestCanManageEnvironments_Unit(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{RoleAdmin, true},
		{RoleManager, true},
		{RoleMember, false},
	}
	for _, tt := range tests {
		if got := CanManageEnvironments(tt.role); got != tt.want {
			t.Errorf("CanManageEnvironments(%q) = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestCanViewAllEnvironments_Unit(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{RoleAdmin, true},
		{RoleManager, true},
		{RoleMember, false},
	}
	for _, tt := range tests {
		if got := CanViewAllEnvironments(tt.role); got != tt.want {
			t.Errorf("CanViewAllEnvironments(%q) = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestCanInvite_Unit(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{RoleAdmin, true},
		{RoleManager, true},
		{RoleMember, false},
	}
	for _, tt := range tests {
		if got := CanInvite(tt.role); got != tt.want {
			t.Errorf("CanInvite(%q) = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestCanListOrgMembers_Unit(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{RoleAdmin, true},
		{RoleManager, true},
		{RoleMember, false},
	}
	for _, tt := range tests {
		if got := CanListOrgMembers(tt.role); got != tt.want {
			t.Errorf("CanListOrgMembers(%q) = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestCanAssignInviteRole_Unit(t *testing.T) {
	tests := []struct {
		inviterRole string
		targetRole  string
		want        bool
	}{
		{RoleAdmin, RoleAdmin, true},
		{RoleAdmin, RoleManager, true},
		{RoleAdmin, RoleMember, true},
		{RoleManager, RoleAdmin, false},
		{RoleManager, RoleManager, false},
		{RoleManager, RoleMember, true},
		{RoleMember, RoleAdmin, false},
		{RoleMember, RoleManager, false},
		{RoleMember, RoleMember, false},
		{RoleAdmin, "invalid", false},
		{RoleManager, "invalid", false},
	}
	for _, tt := range tests {
		if got := CanAssignInviteRole(tt.inviterRole, tt.targetRole); got != tt.want {
			t.Errorf("CanAssignInviteRole(%q, %q) = %v, want %v", tt.inviterRole, tt.targetRole, got, tt.want)
		}
	}
}
