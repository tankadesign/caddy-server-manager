package site

// SiteCreateOptions represents options for creating a site
type SiteCreateOptions struct {
	Domain     string
	WordPress  bool
	DBName     string
	DBPassword string
	MaxUpload  string
	PHPVersion string
}

// SiteDeleteOptions represents options for deleting a site
type SiteDeleteOptions struct {
	Domain     string
	Hard       bool
	Force      bool
}

// Manager interface defines the operations that both managers must implement
type Manager interface {
	CreateSite(opts *SiteCreateOptions) error
	DeleteSite(opts *SiteDeleteOptions) error
	EnableSite(domain string) error
	DisableSite(domain string) error
	ListSites() error
	AddBasicAuth(domain, path, username, password string) error
	RemoveBasicAuth(domain, path string) error
	ListBasicAuth(domain string) error
	ModifyMaxUpload(domain, newSize string) error
}
