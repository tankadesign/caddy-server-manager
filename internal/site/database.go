package site

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Database helper methods

// databaseExists checks if a database exists
func (sm *CaddySiteManager) databaseExists(dbName string) (bool, error) {
	cmd := exec.Command("mysql", "-u", "root", "-e", fmt.Sprintf("SHOW DATABASES LIKE '%s';", dbName))
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	
	return strings.Contains(string(output), dbName), nil
}

// databaseUserExists checks if a database user exists
func (sm *CaddySiteManager) databaseUserExists(dbUser string) (bool, error) {
	query := fmt.Sprintf("SELECT User FROM mysql.user WHERE User='%s' AND Host='localhost';", dbUser)
	cmd := exec.Command("mysql", "-u", "root", "-e", query)
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	
	return strings.Contains(string(output), dbUser), nil
}

// dropDatabase drops a database
func (sm *CaddySiteManager) dropDatabase(dbName string) error {
	query := fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", dbName)
	cmd := exec.Command("mysql", "-u", "root", "-e", query)
	return cmd.Run()
}

// dropDatabaseUser drops a database user
func (sm *CaddySiteManager) dropDatabaseUser(dbUser string) error {
	query := fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost';", dbUser)
	cmd := exec.Command("mysql", "-u", "root", "-e", query)
	return cmd.Run()
}

// deleteDatabase deletes a WordPress database and user
func (sm *CaddySiteManager) deleteDatabase(site *CaddySite) error {
	if !site.IsWordPress {
		if sm.Config.Verbose {
			fmt.Println("Basic PHP site detected - skipping database deletion")
		}
		return nil
	}

	// Extract database info from wp-config.php
	wpConfig := fmt.Sprintf("%s/wp-config.php", site.DocumentRoot)
	dbInfo, err := sm.extractWPDBInfo(wpConfig)
	if err != nil {
		if sm.Config.Verbose {
			fmt.Printf("Could not extract database info from wp-config.php: %v\n", err)
		}
		return nil
	}

	if sm.Config.Verbose {
		fmt.Printf("Deleting WordPress database '%s' and user '%s'...\n", dbInfo.Name, dbInfo.User)
	}

	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would delete database: %s and user: %s\n", dbInfo.Name, dbInfo.User)
		}
		return nil
	}

	// Drop database and user
	queries := []string{
		fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", dbInfo.Name),
		fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost';", dbInfo.User),
		fmt.Sprintf("DROP USER IF EXISTS '%s'@'%s';", dbInfo.User, dbInfo.Host),
		"FLUSH PRIVILEGES;",
	}

	for _, query := range queries {
		cmd := exec.Command("mysql", "-e", query)
		// Don't fail on user drop errors as they might not exist
		cmd.Run()
	}

	if sm.Config.Verbose {
		fmt.Println("WordPress database and user deleted successfully")
	}

	return nil
}

// WPDBInfo holds WordPress database information
type WPDBInfo struct {
	Name     string
	User     string
	Password string
	Host     string
}

// extractWPDBInfo extracts database information from wp-config.php
func (sm *CaddySiteManager) extractWPDBInfo(wpConfigPath string) (*WPDBInfo, error) {
	content, err := os.ReadFile(wpConfigPath)
	if err != nil {
		return nil, fmt.Errorf("could not read wp-config.php: %v", err)
	}

	dbInfo := &WPDBInfo{Host: "localhost"}
	contentStr := string(content)

	// Extract database name
	if match := extractDefine(contentStr, "DB_NAME"); match != "" {
		dbInfo.Name = match
	}

	// Extract database user
	if match := extractDefine(contentStr, "DB_USER"); match != "" {
		dbInfo.User = match
	}

	// Extract database password
	if match := extractDefine(contentStr, "DB_PASSWORD"); match != "" {
		dbInfo.Password = match
	}

	// Extract database host
	if match := extractDefine(contentStr, "DB_HOST"); match != "" {
		dbInfo.Host = match
	}

	if dbInfo.Name == "" || dbInfo.User == "" {
		return nil, fmt.Errorf("could not extract database information")
	}

	return dbInfo, nil
}

// extractDefine extracts a define value from PHP content
func extractDefine(content, defineName string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, fmt.Sprintf("define( '%s'", defineName)) ||
		   strings.Contains(line, fmt.Sprintf("define('%s'", defineName)) ||
		   strings.Contains(line, fmt.Sprintf(`define( "%s"`, defineName)) ||
		   strings.Contains(line, fmt.Sprintf(`define("%s"`, defineName)) {
			
			// Extract the value between quotes
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				value := strings.TrimSpace(parts[1])
				value = strings.Trim(value, " '\"();")
				return value
			}
		}
	}
	return ""
}
