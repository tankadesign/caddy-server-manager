package site

import (
	"fmt"

	"github.com/tankadesign/caddy-site-manager/internal/config"
	"github.com/tankadesign/caddy-site-manager/internal/database"
)

// NewManager creates the SQLite-based site manager
func NewManager(cfg *config.CaddyConfig) (Manager, error) {
	// Create SQLite database connection
	db, err := database.NewDB(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create database connection: %v", err)
	}

	// Create SQLite-based manager
	return NewSQLiteSiteManager(cfg, db)
}
