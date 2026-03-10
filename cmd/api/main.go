// Package main runs the Nervum API server: config, database, migrations, Gin router,
// and feature handlers. Used by the SaaS backend binary (cmd/api).
package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/nervum/nervum-go/internal/config"
	"github.com/nervum/nervum-go/internal/database"
	"github.com/nervum/nervum-go/internal/features/auth"
	"github.com/nervum/nervum-go/internal/features/environments"
	"github.com/nervum/nervum-go/internal/features/entities"
	"github.com/nervum/nervum-go/internal/features/integrations"
	"github.com/nervum/nervum-go/internal/features/invitations"
	"github.com/nervum/nervum-go/internal/features/organizations"
	"github.com/nervum/nervum-go/internal/features/relationships"
	"github.com/nervum/nervum-go/internal/features/repositories"
	"github.com/nervum/nervum-go/internal/features/teams"
	"github.com/nervum/nervum-go/internal/features/user_environment_access"
	"github.com/nervum/nervum-go/internal/features/user_teams"
	"github.com/nervum/nervum-go/internal/features/users"
)

func main() {
	// Load .env from current directory so config sees GITHUB_CLIENT_ID, etc.
	_ = godotenv.Load()

	// Load config, connect DB, run migrations, register routes, and listen.
	cfg := config.Load()
	db, err := database.NewDB(&cfg.Database)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	userRepo := user.NewRepository(db)
	orgRepo := organization.NewRepository(db)
	entityRepo := entity.NewRepository(db)
	sessionRepo := auth.NewSessionRepository(db)
	requireAuth := auth.RequireAuth(sessionRepo, userRepo)

	api := r.Group("/api/v1")

	// Public auth routes
	auth.NewHandler(sessionRepo, userRepo, orgRepo).Register(api)

	// Protected routes
	protected := api.Group("")
	protected.Use(requireAuth)
	organization.NewHandler(orgRepo).Register(protected)
	user.NewHandler(userRepo).Register(protected)
	teamRepo := teams.NewRepository(db)
	userTeamRepo := userteam.NewRepository(db)
	teams.NewHandler(teamRepo, userTeamRepo).Register(protected)
	userteam.NewHandler(userTeamRepo).Register(protected)
	invitationRepo := invitation.NewRepository(db)
	userEnvAccessRepo := userenvironmentaccess.NewRepository(db)
	invHandler := invitation.NewHandler(invitationRepo, userRepo, orgRepo, userTeamRepo, userEnvAccessRepo, sessionRepo)
	invHandler.Register(protected)
	invHandler.RegisterPublic(api)
	environment.NewHandler(environment.NewRepository(db), entityRepo).Register(protected)
	entity.NewHandler(entityRepo).Register(protected)
	relationship.NewHandler(relationship.NewRepository(db)).Register(protected)
	userenvironmentaccess.NewHandler(userenvironmentaccess.NewRepository(db)).Register(protected)
	integrationRepo := integrations.NewRepository(db)
	integHandler := integrations.NewHandler(integrationRepo, orgRepo, &cfg.Integrations)
	integHandler.Register(protected)
	integHandler.RegisterPublic(api)
	dashboardHandler := integrations.NewDashboardHandler(integrationRepo, orgRepo, &cfg.Integrations)
	dashboardHandler.Register(protected.Group("/organizations"))
	repositoriesHandler := repositories.NewHandler(repositories.NewRepository(db))
	repositoriesHandler.Register(protected.Group("/organizations"))

	addr := ":" + fmt.Sprint(cfg.Server.Port)
	log.Printf("listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
