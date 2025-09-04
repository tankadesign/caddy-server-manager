package database

import (
	"time"
)

// Site represents a website configuration in the database
type Site struct {
	ID               int       `db:"id" json:"id"`
	Domain           string    `db:"domain" json:"domain"`
	DocumentRoot     string    `db:"document_root" json:"document_root"`
	PHPVersion       string    `db:"php_version" json:"php_version"`
	IsWordPress      bool      `db:"is_wordpress" json:"is_wordpress"`
	IsEnabled        bool      `db:"is_enabled" json:"is_enabled"`
	MaxUpload        string    `db:"max_upload" json:"max_upload"`
	DBName           string    `db:"db_name" json:"db_name"`
	DBUser           string    `db:"db_user" json:"db_user"`
	DBPassword       string    `db:"db_password" json:"db_password"`
	PoolName         string    `db:"pool_name" json:"pool_name"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
}

// BasicAuth represents basic authentication settings for a site
type BasicAuth struct {
	ID       int    `db:"id" json:"id"`
	SiteID   int    `db:"site_id" json:"site_id"`
	Path     string `db:"path" json:"path"`
	Username string `db:"username" json:"username"`
	Password string `db:"password" json:"password"` // bcrypt hashed
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// SiteWithAuth represents a site with its basic auth configurations
type SiteWithAuth struct {
	Site
	BasicAuths []BasicAuth `json:"basic_auths"`
}
