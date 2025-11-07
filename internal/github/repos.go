package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/s-samadi/ghas-lab-builder/internal/config"
)

func (org *Organization) CreateRepoFromTemplate(ctx context.Context, logger *slog.Logger, templateRepo string, includeAllBranches bool) (*Repository, error) {
	// Enrich context with org-specific information for auth scoping
	ctx = context.WithValue(ctx, config.OrgKey, org.Login)
	return org.createRepoFromTemplateWithRetry(ctx, logger, templateRepo, includeAllBranches, 0)
}

func (org *Organization) createRepoFromTemplateWithRetry(ctx context.Context, logger *slog.Logger, templateRepo string, includeAllBranches bool, retryCount int) (*Repository, error) {
	logger.Info("Creating repository from template",
		slog.String("template", templateRepo),
		slog.Bool("include_all_branches", includeAllBranches))
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// Parse template repo (format: owner/repo)
	parts := strings.Split(templateRepo, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid template repo format, expected 'owner/repo', got: %s", templateRepo)
	}
	templateOwner := parts[0]
	templateRepoName := parts[1]

	baseURL := ctx.Value(config.BaseURLKey).(string)
	apiURL := fmt.Sprintf("%s/repos/%s/%s/generate", baseURL, templateOwner, templateRepoName)

	// Request payload for creating a repo from template
	payload := map[string]interface{}{
		"owner":                org.Login,
		"name":                 templateRepoName,
		"description":          fmt.Sprintf("Repository created from template %s", templateRepo),
		"include_all_branches": includeAllBranches,
		"private":              true,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		logger.Error("Failed to marshal request payload", slog.Any("error", err))
		return nil, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	//user customrt to inject headers
	rt := NewGithubStyleTransport(ctx, logger, config.OrganizationType)

	client := &http.Client{
		Transport: rt,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Error("Failed to create request", slog.Any("error", err))
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to execute request", slog.Any("error", err))
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read response body", slog.Any("error", err))
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		if resp.StatusCode == 422 {
			var errResp struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(body, &errResp); err == nil && strings.Contains(errResp.Message, "Resource not accessible by integration") {
				retryCount++
				logger.Warn("Rate limit hit, retrying after delay",
					slog.Int("retry_count", retryCount))

				logger.Debug("Sleeping for 60 seconds before retry")
				time.Sleep(60 * time.Second)
				return org.createRepoFromTemplateWithRetry(ctx, logger, templateRepo, includeAllBranches, retryCount)
			}
		}
		logger.Error("Failed to create repository from template",
			slog.Int("status_code", resp.StatusCode),
			slog.String("response", string(body)))
		return nil, fmt.Errorf("failed to create repository from template with status %d: %s", resp.StatusCode, string(body))
	}

	var result Repository

	if err := json.Unmarshal(body, &result); err != nil {
		logger.Error("Failed to parse response", slog.Any("error", err))
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	logger.Info("Successfully created repository from template",
		slog.Any("repository", result.FullName),
		slog.Any("url", result.HTMLURL))

	return &result, nil
}

// DeleteRepository deletes a repository in the organization
func (org *Organization) DeleteRepository(ctx context.Context, logger *slog.Logger, repoName string) error {
	logger.Info("Deleting repository",
		slog.String("repo", repoName),
		slog.String("org", org.Name))

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	baseURL := ctx.Value(config.BaseURLKey).(string)
	apiURL := fmt.Sprintf("%s/repos/%s/%s", baseURL, org.Name, repoName)

	rt := NewGithubStyleTransport(ctx, logger, config.OrganizationType)
	client := &http.Client{
		Transport: rt,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, apiURL, nil)
	if err != nil {
		logger.Error("Failed to create request", slog.Any("error", err))
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to execute request", slog.Any("error", err))
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		logger.Error("Failed to delete repository",
			slog.Int("status_code", resp.StatusCode),
			slog.String("response", string(body)))
		return fmt.Errorf("failed to delete repository with status %d: %s", resp.StatusCode, string(body))
	}

	logger.Info("Successfully deleted repository",
		slog.String("repo", repoName),
		slog.String("org", org.Name))

	return nil
}

// ListRepositories lists all repositories in the organization
func (org *Organization) ListRepositories(ctx context.Context, logger *slog.Logger) ([]string, error) {
	logger.Info("Listing repositories in organization", slog.String("org", org.Name))

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	baseURL := ctx.Value(config.BaseURLKey).(string)

	var allRepos []string
	page := 1
	perPage := 100

	rt := NewGithubStyleTransport(ctx, logger, config.OrganizationType)
	client := &http.Client{
		Transport: rt,
	}

	for {
		apiURL := fmt.Sprintf("%s/orgs/%s/repos?per_page=%d&page=%d&type=all", baseURL, org.Name, perPage, page)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			logger.Error("Failed to create request", slog.Any("error", err))
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			logger.Error("Failed to execute request", slog.Any("error", err))
			return nil, fmt.Errorf("failed to execute request: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			logger.Error("Failed to read response body", slog.Any("error", err))
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			logger.Error("Failed to list repositories",
				slog.Int("status_code", resp.StatusCode),
				slog.String("response", string(body)))
			return nil, fmt.Errorf("failed to list repositories with status %d: %s", resp.StatusCode, string(body))
		}

		var repos []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(body, &repos); err != nil {
			logger.Error("Failed to parse response", slog.Any("error", err))
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		// If no repos returned, we've reached the end
		if len(repos) == 0 {
			break
		}

		// Add repo names to the list
		for _, repo := range repos {
			allRepos = append(allRepos, repo.Name)
		}

		// If we got fewer repos than requested, we're done
		if len(repos) < perPage {
			break
		}

		page++
	}

	logger.Info("Found repositories",
		slog.Int("count", len(allRepos)),
		slog.String("org", org.Name))

	return allRepos, nil
}
