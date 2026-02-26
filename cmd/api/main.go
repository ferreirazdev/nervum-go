package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nervum/nervum-go/internal/config"
	"github.com/nervum/nervum-go/internal/database"
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
	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	api := r.Group("/api/v1")
	organization.NewHandler(organization.NewRepository(db)).Register(api)
	user.NewHandler(user.NewRepository(db)).Register(api)
	environment.NewHandler(environment.NewRepository(db)).Register(api)
	entity.NewHandler(entity.NewRepository(db)).Register(api)
	relationship.NewHandler(relationship.NewRepository(db)).Register(api)
	userenvironmentaccess.NewHandler(userenvironmentaccess.NewRepository(db)).Register(api)

	addr := ":" + fmt.Sprint(cfg.Server.Port)
	log.Printf("listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
