# Caddy Site Manager

A powerful CLI tool for managing PHP and WordPress sites with Caddy web server. Automates site creation, configuration, and management with custom PHP-FPM pools for optimal performance and isolation.

## Features

- ✅ **Automated Site Creation**: Create PHP and WordPress sites with one command
- ✅ **Custom PHP-FPM Pools**: Each site gets its own isolated PHP-FPM pool
- ✅ **WordPress Support**: Automatic database creation and wp-config.php generation
- ✅ **Caddy Integration**: Generates optimized Caddy configurations
- ✅ **Site Management**: Enable, disable, list, and delete sites
- ✅ **Dry Run Mode**: Test commands without making changes
- ✅ **Configurable**: Support for custom PHP versions, upload limits, and paths
- ✅ **Security Focused**: Proper file permissions and security headers

## Installation

### From Source

```bash
git clone <repository-url>
cd caddy-site-manager-golang
go build -o build/caddy-site-manager
```

### Pre-built Binaries

Download the appropriate binary for your platform from the releases page.

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
├── available-sites/     # Site configurations
├── enabled-sites/       # Symlinks to enabled sites
└── Caddyfile           # Main Caddy config

/var/www/
├── sites/              # Site document roots
└── sites/wordpress-template/  # WordPress template (for WP sites)
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

Example for WordPress:

```caddy
blog.example.com {
    root * /var/www/sites/blog.example.com
    encode gzip

    request_body {
        max_size 256M
    }

    php_fastcgi unix//run/php/php8.3-fpm-blog_example_com.sock {
        index index.php
    }

    try_files {path} {path}/ /index.php?{query}

    @forbidden {
        path *.sql
        path /wp-config.php
        path /wp-content/debug.log
    }
    respond @forbidden 403

    header {
        -Server
        X-Content-Type-Options nosniff
        X-XSS-Protection "1; mode=block"
    }

    file_server
}
```

## WordPress Features

When creating WordPress sites (`--wordpress` flag):

- 🔐 **Automatic Database Creation**: Creates MySQL database and user
- 🔑 **Secure wp-config.php**: Generates configuration with WordPress salts
- 📁 **Template System**: Copies from WordPress template directory
- 🛡️ **Security**: Proper file permissions and security headers
- 📊 **Database Credentials**: Displays credentials for WordPress installation

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
