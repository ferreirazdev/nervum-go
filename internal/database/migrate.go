package database

import (
	"log"

	"github.com/nervum/nervum-go/internal/features/environments"
	"github.com/nervum/nervum-go/internal/features/entities"
	"github.com/nervum/nervum-go/internal/features/organizations"
	"github.com/nervum/nervum-go/internal/features/relationships"
	"github.com/nervum/nervum-go/internal/features/user_environment_access"
	"github.com/nervum/nervum-go/internal/features/users"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&organization.Organization{},
		&user.User{},
		&environment.Environment{},
		&entity.Entity{},
		&relationship.Relationship{},
		&userenvironmentaccess.UserEnvironmentAccess{},
	); err != nil {
		return err
	}
	log.Println("migrations completed")
	return nil
}
