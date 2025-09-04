# WordPress Module

This module handles the automatic download, extraction, and configuration of WordPress installations for the Caddy Site Manager.

## Features

- **Latest WordPress**: Automatically downloads the latest WordPress version from wordpress.org
- **Secure Configuration**: Generates wp-config.php with security best practices
- **Security Hardening**: Implements comprehensive WordPress security measures
- **No Template Required**: Eliminates the need for pre-existing WordPress template directories

## Security Features

### wp-config.php Security

- Disables file editing in WordPress admin (`DISALLOW_FILE_EDIT`)
- Implements proper memory limits and timeout settings
- Forces SSL for admin areas (`FORCE_SSL_ADMIN`)
- Configures secure cookie settings
- Sets up proper debug settings for production

### Authentication & Salts

- Fetches fresh authentication keys and salts from WordPress.org API
- Falls back to cryptographically secure local generation if API is unavailable
- Generates additional custom security tokens for each installation

### File System Security

- Proper file permissions (600 for wp-config.php)
- Protects sensitive files (wp-config.php, WordPress core files, etc.)
- Prevents PHP execution in upload directories via Caddy configuration
- Implements proper file permissions (600 for wp-config.php)

### SEO & Crawling

- Generates robots.txt with WordPress-appropriate rules
- Allows access to necessary assets while protecting admin areas

## Usage

The WordPress module is automatically used when creating WordPress sites:

```bash
# The CLI tool will automatically use this module for WordPress sites
caddy-site-manager create myblog.com --wordpress
```

## Implementation Details

### Download Process

1. Downloads `https://wordpress.org/latest.tar.gz` directly
2. Streams and extracts the archive to the target directory
3. Removes the "wordpress/" prefix from paths during extraction
4. Validates the installation by checking for required files

### Configuration Process

1. Generates secure wp-config.php with database credentials
2. Fetches or generates authentication keys and salts
3. Adds security hardening configurations
4. Generates robots.txt for SEO

### Error Handling

- Automatic cleanup of partial installations on errors
- Validation of required WordPress files
- Graceful fallback for API failures

## Files Created

- `index.php` - WordPress bootstrap file
- `wp-config.php` - Secure WordPress configuration (600 permissions)
- `robots.txt` - SEO-friendly crawling rules
- All WordPress core files and directories

## Security Best Practices

This module implements the latest WordPress security recommendations:

1. **File Permissions**: Proper file and directory permissions
2. **Security Headers**: Via Caddy configuration and directives
3. **File Protection**: Prevents access to sensitive files via Caddy rules
4. **Upload Security**: Prevents PHP execution in upload directories
5. **Admin Security**: Forces SSL and disables file editing
6. **Database Security**: Uses unique credentials per installation

## Future Enhancements

Potential improvements for future versions:

- WordPress plugin installation
- Automatic SSL certificate generation
- WordPress CLI integration
- Backup and restore functionality
- Staging environment creation
