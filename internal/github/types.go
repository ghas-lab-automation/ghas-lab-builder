package api

// Enterprise represents the enterprise information returned from GitHub GraphQL API
type Enterprise struct {
	ID           string `json:"id"`
	BillingEmail string `json:"billingEmail"`
	Slug         string `json:"slug"`
}

type Organization struct {
	ID    string `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
}

type Repository struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url"`
}

type AppInstallation struct {
	ID                  int64  `json:"id"`
	AppID               int64  `json:"app_id"`
	AppSlug             string `json:"app_slug"`
	TargetID            int64  `json:"target_id"`
	TargetType          string `json:"target_type"`
	RepositorySelection string `json:"repository_selection,omitempty"`
	Account             struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"account"`
}
