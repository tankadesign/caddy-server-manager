package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB represents the database connection
type DB struct {
	conn *sql.DB
	path string
}

// NewDB creates a new database connection
func NewDB(dbPath string) (*DB, error) {
	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %v", err)
	}

	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	db := &DB{
		conn: conn,
		path: dbPath,
	}

	// Initialize the database schema
	if err := db.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize database schema: %v", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// initSchema creates the necessary tables
func (db *DB) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS sites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			domain TEXT UNIQUE NOT NULL,
			document_root TEXT NOT NULL,
			php_version TEXT NOT NULL DEFAULT '8.1',
			is_wordpress BOOLEAN NOT NULL DEFAULT FALSE,
			is_enabled BOOLEAN NOT NULL DEFAULT FALSE,
			max_upload TEXT NOT NULL DEFAULT '256M',
			db_name TEXT,
			db_user TEXT,
			db_password TEXT,
			pool_name TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS basic_auths (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id INTEGER NOT NULL,
			path TEXT NOT NULL,
			username TEXT NOT NULL,
			password TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (site_id) REFERENCES sites(id) ON DELETE CASCADE,
			UNIQUE(site_id, path, username)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sites_domain ON sites(domain)`,
		`CREATE INDEX IF NOT EXISTS idx_sites_enabled ON sites(is_enabled)`,
		`CREATE INDEX IF NOT EXISTS idx_basic_auths_site_id ON basic_auths(site_id)`,
		`CREATE INDEX IF NOT EXISTS idx_basic_auths_path ON basic_auths(site_id, path)`,
	}

	for _, query := range queries {
		if _, err := db.conn.Exec(query); err != nil {
			return fmt.Errorf("failed to execute schema query: %v", err)
		}
	}

	return nil
}

// Site operations

// CreateSite creates a new site in the database
func (db *DB) CreateSite(site *Site) error {
	site.CreatedAt = time.Now()
	site.UpdatedAt = time.Now()

	query := `INSERT INTO sites (
		domain, document_root, php_version, is_wordpress, is_enabled, max_upload,
		db_name, db_user, db_password, pool_name, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := db.conn.Exec(query,
		site.Domain, site.DocumentRoot, site.PHPVersion, site.IsWordPress, site.IsEnabled,
		site.MaxUpload, site.DBName, site.DBUser, site.DBPassword, site.PoolName,
		site.CreatedAt, site.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create site: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get site ID: %v", err)
	}

	site.ID = int(id)
	return nil
}

// GetSite retrieves a site by domain
func (db *DB) GetSite(domain string) (*Site, error) {
	query := `SELECT id, domain, document_root, php_version, is_wordpress, is_enabled,
		max_upload, db_name, db_user, db_password, pool_name, created_at, updated_at
		FROM sites WHERE domain = ?`

	var site Site
	err := db.conn.QueryRow(query, domain).Scan(
		&site.ID, &site.Domain, &site.DocumentRoot, &site.PHPVersion, &site.IsWordPress,
		&site.IsEnabled, &site.MaxUpload, &site.DBName, &site.DBUser, &site.DBPassword,
		&site.PoolName, &site.CreatedAt, &site.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("site not found: %s", domain)
		}
		return nil, fmt.Errorf("failed to get site: %v", err)
	}

	return &site, nil
}

// GetSiteWithAuth retrieves a site with its basic auth configurations
func (db *DB) GetSiteWithAuth(domain string) (*SiteWithAuth, error) {
	site, err := db.GetSite(domain)
	if err != nil {
		return nil, err
	}

	auths, err := db.GetBasicAuths(site.ID)
	if err != nil {
		return nil, err
	}

	return &SiteWithAuth{
		Site:       *site,
		BasicAuths: auths,
	}, nil
}

// UpdateSite updates an existing site
func (db *DB) UpdateSite(site *Site) error {
	site.UpdatedAt = time.Now()

	query := `UPDATE sites SET
		document_root = ?, php_version = ?, is_wordpress = ?, is_enabled = ?,
		max_upload = ?, db_name = ?, db_user = ?, db_password = ?, pool_name = ?,
		updated_at = ?
		WHERE domain = ?`

	_, err := db.conn.Exec(query,
		site.DocumentRoot, site.PHPVersion, site.IsWordPress, site.IsEnabled,
		site.MaxUpload, site.DBName, site.DBUser, site.DBPassword, site.PoolName,
		site.UpdatedAt, site.Domain,
	)
	if err != nil {
		return fmt.Errorf("failed to update site: %v", err)
	}

	return nil
}

// DeleteSite deletes a site and all its basic auth configurations
func (db *DB) DeleteSite(domain string) error {
	query := `DELETE FROM sites WHERE domain = ?`
	_, err := db.conn.Exec(query, domain)
	if err != nil {
		return fmt.Errorf("failed to delete site: %v", err)
	}
	return nil
}

// ListSites returns all sites, optionally filtered by enabled status
func (db *DB) ListSites(enabledOnly *bool) ([]Site, error) {
	var query string
	var args []interface{}

	if enabledOnly != nil {
		query = `SELECT id, domain, document_root, php_version, is_wordpress, is_enabled,
			max_upload, db_name, db_user, db_password, pool_name, created_at, updated_at
			FROM sites WHERE is_enabled = ? ORDER BY domain`
		args = append(args, *enabledOnly)
	} else {
		query = `SELECT id, domain, document_root, php_version, is_wordpress, is_enabled,
			max_upload, db_name, db_user, db_password, pool_name, created_at, updated_at
			FROM sites ORDER BY domain`
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list sites: %v", err)
	}
	defer rows.Close()

	var sites []Site
	for rows.Next() {
		var site Site
		err := rows.Scan(
			&site.ID, &site.Domain, &site.DocumentRoot, &site.PHPVersion, &site.IsWordPress,
			&site.IsEnabled, &site.MaxUpload, &site.DBName, &site.DBUser, &site.DBPassword,
			&site.PoolName, &site.CreatedAt, &site.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan site: %v", err)
		}
		sites = append(sites, site)
	}

	return sites, nil
}

// Basic Auth operations

// CreateBasicAuth creates a new basic auth configuration
func (db *DB) CreateBasicAuth(auth *BasicAuth) error {
	auth.CreatedAt = time.Now()
	auth.UpdatedAt = time.Now()

	query := `INSERT INTO basic_auths (site_id, path, username, password, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`

	result, err := db.conn.Exec(query,
		auth.SiteID, auth.Path, auth.Username, auth.Password,
		auth.CreatedAt, auth.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create basic auth: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get basic auth ID: %v", err)
	}

	auth.ID = int(id)
	return nil
}

// GetBasicAuths retrieves all basic auth configurations for a site
func (db *DB) GetBasicAuths(siteID int) ([]BasicAuth, error) {
	query := `SELECT id, site_id, path, username, password, created_at, updated_at
		FROM basic_auths WHERE site_id = ? ORDER BY path, username`

	rows, err := db.conn.Query(query, siteID)
	if err != nil {
		return nil, fmt.Errorf("failed to get basic auths: %v", err)
	}
	defer rows.Close()

	var auths []BasicAuth
	for rows.Next() {
		var auth BasicAuth
		err := rows.Scan(
			&auth.ID, &auth.SiteID, &auth.Path, &auth.Username, &auth.Password,
			&auth.CreatedAt, &auth.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan basic auth: %v", err)
		}
		auths = append(auths, auth)
	}

	return auths, nil
}

// DeleteBasicAuth deletes a basic auth configuration
func (db *DB) DeleteBasicAuth(siteID int, path, username string) error {
	query := `DELETE FROM basic_auths WHERE site_id = ? AND path = ? AND username = ?`
	result, err := db.conn.Exec(query, siteID, path, username)
	if err != nil {
		return fmt.Errorf("failed to delete basic auth: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("basic auth not found for site ID %d, path %s, username %s", siteID, path, username)
	}

	return nil
}

// DeleteBasicAuthsForPath deletes all basic auth configurations for a specific path
func (db *DB) DeleteBasicAuthsForPath(siteID int, path string) error {
	query := `DELETE FROM basic_auths WHERE site_id = ? AND path = ?`
	_, err := db.conn.Exec(query, siteID, path)
	if err != nil {
		return fmt.Errorf("failed to delete basic auths for path: %v", err)
	}
	return nil
}

// Utility methods

// SiteExists checks if a site exists in the database
func (db *DB) SiteExists(domain string) (bool, error) {
	query := `SELECT COUNT(*) FROM sites WHERE domain = ?`
	var count int
	err := db.conn.QueryRow(query, domain).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check site existence: %v", err)
	}
	return count > 0, nil
}
