# Build Instructions for Caddy Site Manager

## Development Build

For development and local testing:

```bash
# Build for current platform
go build

# Run with verbose output
./caddy-site-manager --help
```

## Production Builds

### Cross-Platform Compilation

The Go compiler supports cross-compilation to build binaries for different operating systems and architectures.

#### Linux x64 (Most common for servers)

```bash
# For Ubuntu/Debian servers
GOOS=linux GOARCH=amd64 go build -o caddy-site-manager-linux-amd64

# For production servers (optimized build)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o caddy-site-manager-linux-amd64
```

#### Linux ARM64 (For ARM-based servers)

```bash
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o caddy-site-manager-linux-arm64
```

#### Build Flags Explanation

- `-ldflags="-s -w"`: Strips debug information and symbol tables to reduce binary size
- `-o filename`: Specifies the output binary name

### Deployment to Remote Server

1. **Build for your target server:**
   ```bash
   GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o caddy-site-manager-linux
   ```

2. **Copy to server:**
   ```bash
   scp caddy-site-manager-linux user@your-server:/usr/local/bin/caddy-site-manager
   ```

3. **Set permissions on server:**
   ```bash
   ssh user@your-server "chmod +x /usr/local/bin/caddy-site-manager"
   ```

4. **Test on server:**
   ```bash
   ssh user@your-server "caddy-site-manager --help"
   ```

### Server Requirements

Your server needs:
- **Caddy web server** installed and running
- **PHP-FPM** (version 8.1+ recommended)
- **MySQL/MariaDB** (for WordPress sites)
- **Proper directory structure:**
  ```
  /etc/caddy/
  ├── available-sites/
  ├── enabled-sites/
  └── Caddyfile
  
  /var/www/
  └── sites/
  ```

### Installation Script for Server

Create this script on your server to set up the environment:

```bash
#!/bin/bash
# setup-caddy-manager.sh

# Create directory structure
sudo mkdir -p /etc/caddy/{available-sites,enabled-sites}
sudo mkdir -p /var/www/sites
sudo mkdir -p /var/www/sites/wordpress-template

# Set permissions
sudo chown -R www-data:www-data /var/www/sites
sudo chmod -R 755 /var/www/sites

# Copy the binary
sudo cp caddy-site-manager-linux /usr/local/bin/caddy-site-manager
sudo chmod +x /usr/local/bin/caddy-site-manager

echo "Caddy Site Manager installed successfully!"
echo "Run: caddy-site-manager --help"
```

### WordPress Template Setup

For WordPress sites, create a template directory:

```bash
# Download WordPress
cd /var/www/sites
sudo wget https://wordpress.org/latest.tar.gz
sudo tar -xzf latest.tar.gz
sudo mv wordpress wordpress-template
sudo rm latest.tar.gz

# Set permissions
sudo chown -R www-data:www-data wordpress-template
sudo chmod -R 755 wordpress-template
```

## Configuration

### Default Configuration File

Create `/etc/caddy-site-manager.yaml`:

```yaml
# Caddy Site Manager Configuration
caddy_config: "/etc/caddy"
web_root: "/var/www"
php_version: "8.3"
max_upload: "256M"
```

### System Integration

Add to your server's Caddyfile include directive:

```caddy
# /etc/caddy/Caddyfile
{
    # Global options
    email your-email@domain.com
}

# Import all enabled sites
import enabled-sites/*
```

## Usage Examples

### Create a basic PHP site:
```bash
caddy-site-manager create mysite.com
```

### Create a WordPress site:
```bash
caddy-site-manager create blog.com --wordpress
```

### Create with custom settings:
```bash
caddy-site-manager create bigsite.com --wordpress --max-upload=1G --php=8.2
```

### Test before applying (dry-run):
```bash
caddy-site-manager create test.com --dry-run --verbose
```

### Site management:
```bash
# List all sites
caddy-site-manager list

# Disable a site
caddy-site-manager disable mysite.com

# Enable a site
caddy-site-manager enable mysite.com

# Soft delete (remove from enabled sites)
caddy-site-manager delete mysite.com

# Hard delete (remove everything)
caddy-site-manager delete mysite.com --hard --force
```

## Binary Sizes

Typical binary sizes with optimization flags:
- Linux AMD64: ~13-15 MB
- Linux ARM64: ~12-14 MB

The large size is due to Go's static linking, which includes the entire runtime. This makes deployment easier as there are no dependencies.
