# Caddy Site Manager - Go CLI Tool

This is a completed Go CLI application for managing Caddy PHP/WordPress sites. The project has been successfully ported from bash scripts to a proper Go CLI tool using the Cobra framework.

## Project Status: ✅ COMPLETED

- ✅ **Project Requirements Clarified**: Go CLI application for managing Caddy PHP/WordPress sites
- ✅ **Project Scaffolded**: Complete Go project structure with proper modules and dependencies
- ✅ **Project Customized**: Full implementation of bash script functionality in Go
- ✅ **Extensions**: No additional extensions required - Go tooling sufficient
- ✅ **Compilation**: Project compiles successfully with `go build`
- ✅ **Tasks**: CLI application doesn't require VS Code tasks
- ✅ **Launch**: CLI tool ready for use - no debug launch needed
- ✅ **Documentation**: Complete README.md and BUILD.md documentation provided

## Key Features Implemented

- Complete site creation (PHP and WordPress)
- Site management (enable, disable, delete, list)
- Custom PHP-FPM pool generation
- Caddy configuration generation
- Database management for WordPress
- Dry-run mode for safe testing
- Verbose output and comprehensive error handling
- Configuration file support
- Cross-platform build support

## Usage

```bash
# Build the project
go build -o build/caddy-site-manager

cd build

# Create a PHP site
./caddy-site-manager create example.com

# Create a WordPress site
./caddy-site-manager create blog.com --wordpress

# Test with dry-run
./caddy-site-manager create test.com --dry-run --verbose

# List all sites
./caddy-site-manager list

# Get help
./caddy-site-manager --help
```

## Build for Production

Always build into the build directory.

```bash
# Linux server deployment
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o caddy-site-manager-linux
```

See BUILD.md for detailed production deployment instructions.
