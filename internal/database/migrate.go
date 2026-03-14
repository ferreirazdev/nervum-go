package database

import (
	"log"

	"github.com/nervum/nervum-go/internal/features/auth"
	"github.com/nervum/nervum-go/internal/features/environments"
	"github.com/nervum/nervum-go/internal/features/entities"
	integration "github.com/nervum/nervum-go/internal/features/integrations"
	"github.com/nervum/nervum-go/internal/features/invitations"
	"github.com/nervum/nervum-go/internal/features/organizations"
	"github.com/nervum/nervum-go/internal/features/orgservices"
	"github.com/nervum/nervum-go/internal/features/relationships"
	"github.com/nervum/nervum-go/internal/features/repositories"
	"github.com/nervum/nervum-go/internal/features/teams"
	"github.com/nervum/nervum-go/internal/features/user_environment_access"
	"github.com/nervum/nervum-go/internal/features/user_teams"
	"github.com/nervum/nervum-go/internal/features/users"
	"gorm.io/gorm"
)

// AutoMigrate runs GORM AutoMigrate for all feature models (organizations, users, sessions,
// environments, teams, entities, relationships, invitations, user_environment_access, etc.).
func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&organization.Organization{},
		&user.User{},
		&auth.Session{},
		&environment.Environment{},
		&teams.Team{},
		&teams.TeamEnvironment{},
		&userteam.UserTeam{},
		&invitation.Invitation{},
		&invitation.InvitationTeam{},
		&entity.Entity{},
		&relationship.Relationship{},
		&userenvironmentaccess.UserEnvironmentAccess{},
		&integration.Integration{},
		&repositories.OrganizationRepository{},
		&orgservices.OrganizationService{},
	); err != nil {
		return err
	}
	log.Println("migrations completed")
	return nil
}
