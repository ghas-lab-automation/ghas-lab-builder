package util

import (
	"encoding/json"
	"os"
)

// RepoConfig represents a repository configuration
type RepoConfig struct {
	Template           string `json:"template"`
	IncludeAllBranches bool   `json:"include_all_branches"`
}

// UnmarshalJSON allows RepoConfig to accept both string and object formats
func (r *RepoConfig) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		r.Template = str
		r.IncludeAllBranches = false // default value
		return nil
	}

	// If that fails, try as object
	type Alias RepoConfig
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	return nil
}

type TemplateReposConfig struct {
	LabEnvSetup struct {
		Repos []RepoConfig `json:"repos"`
	} `json:"lab-env-setup"`
}

func LoadFromJsonFile(path string) ([]RepoConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config TemplateReposConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return config.LabEnvSetup.Repos, nil
}
