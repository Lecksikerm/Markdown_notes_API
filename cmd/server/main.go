package main

import (
	"log"

	"markdown-notes/internal/config"
	"markdown-notes/internal/router"
	"markdown-notes/pkg/database"
)

func main() {
	// Load config
	cfg := config.Load()

	// Connect to database
	db, err := database.NewPostgres(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations (simple version - we'll improve later)
	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Setup router
	r := router.New(db)

	// Start server
	log.Printf("Server starting on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}