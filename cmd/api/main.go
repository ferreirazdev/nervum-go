package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/nervum/nervum-go/internal/config"
	"github.com/nervum/nervum-go/internal/database"
	"github.com/nervum/nervum-go/internal/features/auth"
	"github.com/nervum/nervum-go/internal/features/environments"
	"github.com/nervum/nervum-go/internal/features/entities"
	"github.com/nervum/nervum-go/internal/features/organizations"
	"github.com/nervum/nervum-go/internal/features/relationships"
	"github.com/nervum/nervum-go/internal/features/user_environment_access"
	"github.com/nervum/nervum-go/internal/features/users"
)

func main() {
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
	sessionRepo := auth.NewSessionRepository(db)
	requireAuth := auth.RequireAuth(sessionRepo, userRepo)

	api := r.Group("/api/v1")

	// Public auth routes
	auth.NewHandler(sessionRepo, userRepo).Register(api)

	// Protected routes
	protected := api.Group("")
	protected.Use(requireAuth)
	organization.NewHandler(organization.NewRepository(db)).Register(protected)
	user.NewHandler(userRepo).Register(protected)
	environment.NewHandler(environment.NewRepository(db)).Register(protected)
	entity.NewHandler(entity.NewRepository(db)).Register(protected)
	relationship.NewHandler(relationship.NewRepository(db)).Register(protected)
	userenvironmentaccess.NewHandler(userenvironmentaccess.NewRepository(db)).Register(protected)

	addr := ":" + fmt.Sprint(cfg.Server.Port)
	log.Printf("listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
