# Caddy Site Manager

A 100% vibe coded CLI tool written in Go for managing PHP and WordPress sites with Caddy web server. Automates basic site creation, configuration, and management with custom PHP-FPM pools for optimal performance and isolation. It’s probably not very good but it does what I want and that’s good enough for me.

## Features

- ✅ **Automated Site Creation**: Create PHP and WordPress sites with one command
- ✅ **Custom PHP-FPM Pools**: Each site gets its own isolated PHP-FPM pool
- ✅ **WordPress Support**: Latest WordPress auto-download with security hardening
- ✅ **Caddy Integration**: Generates optimized Caddy configurations
- ✅ **Site Management**: Enable, disable, list, and delete sites
- ✅ **Site Modification**: Add/remove basic auth and change upload limits
- ✅ **Basic Authentication**: Secure paths with username/password protection
- ✅ **Upload Size Management**: Modify PHP and Caddy upload limits dynamically
- ✅ **Dry Run Mode**: Test commands without making changes
- ✅ **Configurable**: Support for custom PHP versions, upload limits, and paths

## Installation

### From Source

```bash
git clone <repository-url>
cd caddy-site-manager
go build -o build/caddy-site-manager
```

## Quick Start

1. **Create a basic PHP site:**

   ```bash
   caddy-site-manager create mysite.com
   ```

2. **Create a WordPress site:**

   ```bash
   caddy-site-manager create blog.com --wordpress
   ```

3. **Test without making changes:**
   ```bash
   caddy-site-manager create test.com --dry-run --verbose
   ```

## Commands

### Create Sites

```bash
# Basic PHP site
caddy-site-manager create example.com

# WordPress site with auto-generated database
caddy-site-manager create blog.example.com --wordpress

# WordPress with custom database settings
caddy-site-manager create shop.example.com --wordpress --db=shop_db --pwd=secure123

# Custom PHP version and upload limit
caddy-site-manager create bigsite.com --php=8.2 --max-upload=1G

# Dry run to see what would happen
caddy-site-manager create test.com --dry-run --verbose
```

### Site Management

```bash
# List all sites
caddy-site-manager list

# Enable a site
caddy-site-manager enable mysite.com

# Disable a site
caddy-site-manager disable mysite.com

# Soft delete (removes from enabled sites only)
caddy-site-manager delete mysite.com

# Hard delete (removes everything including files and database)
caddy-site-manager delete mysite.com --hard --force
```

### Site Modification

```bash
# Add basic authentication to a path
caddy-site-manager auth-add example.com "/admin" -u admin -p secret123
caddy-site-manager auth-add blog.com "/wp-admin" -u wordpress -p mypassword

# Remove basic authentication from a path
caddy-site-manager auth-remove example.com "/admin"
caddy-site-manager auth-remove blog.com "/wp-admin"

# Change maximum upload size
caddy-site-manager max-upload example.com 2GB
caddy-site-manager max-upload blog.com 512M
caddy-site-manager max-upload bigsite.com 1G

# Test modifications safely with dry-run
caddy-site-manager auth-add test.com "/secure" -u user -p pass --dry-run --verbose
caddy-site-manager max-upload test.com 2GB --dry-run --verbose
```

### Global Options

```bash
# Use custom Caddy config directory
caddy-site-manager create site.com --caddy-config=/opt/caddy

# Verbose output
caddy-site-manager create site.com --verbose

# Dry run mode
caddy-site-manager create site.com --dry-run

# Use custom config file
caddy-site-manager create site.com --config=/path/to/config.yaml
```

## Configuration

### Configuration File

Create a configuration file at `~/.caddy-site-manager.yaml`:

```yaml
caddy_config: '/etc/caddy'
web_root: '/var/www'
php_version: '8.3'
max_upload: '256M'
```

### Directory Structure

The tool expects this directory structure:

```
/etc/caddy/
├── available-sites/    # Site configurations
├── enabled-sites/      # Symlinks to enabled sites
└── Caddyfile           # Main Caddy config

/var/www/sites/         # Individual site directories
```

## Generated Configurations

### PHP-FPM Pool

Each site gets a custom PHP-FPM pool with optimized settings:

```ini
[site_name]
user = www-data
group = www-data
listen = /run/php/php8.3-fpm-site_name.sock

pm = dynamic
pm.max_children = 10
pm.start_servers = 3
pm.min_spare_servers = 2
pm.max_spare_servers = 5

php_admin_value[upload_max_filesize] = 256M
php_admin_value[post_max_size] = 256M
php_admin_value[memory_limit] = 512M
# ... more optimizations
```

### Caddy Configuration

Generates secure Caddy configurations with:

- PHP-FPM integration using custom sockets
- Security headers
- WordPress-specific rules (if applicable)
- Automatic HTTPS
- Request body limits matching PHP settings

## WordPress Features

When creating WordPress sites (`--wordpress` flag):

- � **Latest WordPress**: Automatically downloads and installs the latest WordPress version
- 🔒 **Security Hardening**: Implements WordPress security best practices out of the box
- 🔑 **Secure wp-config.php**: Advanced configuration with security keys, salts, and hardening
- 🛡️ **Additional Security**: Caddy-native security rules, headers, and file protection
- �️ **Database Setup**: Auto-creates MySQL database and user with proper permissions
- 📊 **Database Credentials**: Displays credentials for WordPress installation
- 🏗️ **No Template Required**: No need for pre-existing WordPress templates or directories

### Security Features Included:

- Disables file editing in WordPress admin
- Implements proper file permissions and ownership
- Configures Caddy-native security directives and headers
- Protects sensitive files (wp-config.php, WordPress core files, etc.)
- Generates cryptographically secure authentication keys and salts
- Creates secure robots.txt for SEO
- Configures proper PHP security settings
- Implements WordPress-specific Caddy rules for protection

## Architecture

The CLI tool is built with a modular architecture:

```
cmd/               # CLI commands (create, enable, disable, etc.)
internal/
├── config/        # Configuration management
├── database/      # SQLite database operations and models
├── site/          # Site management (main business logic)
└── wordpress/     # WordPress-specific operations (NEW)
```

### Key Components

- **Site Manager**: Handles all site operations (PHP and WordPress)
- **WordPress Module**: Dedicated module for WordPress download, extraction, and security configuration
- **Database Layer**: SQLite-based storage for site configurations and metadata
- **Configuration System**: YAML-based configuration with sensible defaults

## Server Requirements

- **Caddy** web server
- **PHP-FPM** (8.1+ recommended)
- **MySQL/MariaDB** (for WordPress sites)
- **Linux environment** with standard utilities (`cp`, `chown`, `find`)

## Security Features

- 🔒 **Isolated PHP-FPM pools** for each site
- 🛡️ **Security headers** in Caddy configurations
- 🔐 **Proper file permissions** (644 for files, 755 for directories)
- 🚫 **Protected sensitive files** (wp-config.php, .htaccess, etc.)
- 🔑 **Random password generation** for databases
- 🌐 **WordPress security salts** from official API

## Development

### Building

```bash
# Development build
go build

# Production build for Linux
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o caddy-site-manager-linux
```

### Project Structure

```
caddy-site-manager-golang/
├── main.go                 # Entry point
├── cmd/                    # CLI commands
│   ├── root.go            # Root command and global flags
│   ├── create.go          # Site creation command
│   └── enable.go          # Site management commands
├── internal/
│   ├── config/            # Configuration management
│   └── site/              # Core site management logic
└── BUILD.md               # Detailed build instructions
```

## Migration from Bash Scripts

This tool is a direct port of bash scripts with these improvements:

- ✅ **Better error handling** with detailed messages
- ✅ **Dry run support** for safe testing
- ✅ **Structured configuration** with YAML support
- ✅ **Cross-platform compatibility**
- ✅ **Improved user experience** with progress indicators
- ✅ **Type safety** and better code organization

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

[Add your license information here]

## Support

For issues and questions:

- Check the documentation in this README
- Review the BUILD.md for deployment instructions
- Use `--verbose` flag for detailed output
- Use `--dry-run` to test commands safely
