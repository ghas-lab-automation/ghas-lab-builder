package config

type contextKey string

const (
	TokenKey          contextKey = "token"
	AppIDKey          contextKey = "app-id"
	PrivateKeyKey     contextKey = "private-key"
	BaseURLKey        contextKey = "base-url"
	EnterpriseSlugKey contextKey = "enterprise-slug"
	LabDateKey        contextKey = "lab-date"
	FacilitatorsKey   contextKey = "facilitators"
	LoggerKey         contextKey = "logger"
	OrgKey            contextKey = "org"
)

const (
	DefaultBaseURL   string = "https://api.github.com"
	EnterpriseType   string = "Enterprise"
	OrganizationType string = "Organization"
)
