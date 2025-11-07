package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/s-samadi/ghas-lab-builder/internal/auth"
	"github.com/s-samadi/ghas-lab-builder/internal/config"
)

func (enterprise *Enterprise) CreateOrg(ctx context.Context, logger *slog.Logger, user string) (*Organization, error) {
	orgName := "ghas-labs-" + ctx.Value(config.LabDateKey).(string) + "-" + user
	logger.Info("Creating organization", slog.String("org", orgName), slog.String("user", user))
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rt := NewGithubStyleTransport(ctx, logger, config.EnterpriseType)

	client := &http.Client{
		Transport: rt,
	}

	baseURL := ctx.Value(config.BaseURLKey).(string)
	graphqlURL := baseURL + "/graphql"

	mutation := `
		mutation($enterpriseId: ID!, $login: String!, $profileName: String!, $adminLogins: [String!]!, $billingEmail: String!) {
			createEnterpriseOrganization(input: {
				enterpriseId: $enterpriseId
				login: $login
				profileName: $profileName
				adminLogins: $adminLogins
				billingEmail: $billingEmail
			}) {
				organization {
					id
					login
					name
				}
			}
		}
	`

	facilitators := ctx.Value(config.FacilitatorsKey).([]string)
	billingEmail := enterprise.BillingEmail
	if billingEmail == "" && len(facilitators) > 0 {
		billingEmail = facilitators[0] + "@github.com"
	}

	payload := map[string]interface{}{
		"query": mutation,
		"variables": map[string]interface{}{
			"enterpriseId": enterprise.ID,
			"login":        orgName,
			"profileName":  orgName,
			"adminLogins":  facilitators,
			"billingEmail": billingEmail,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		logger.Error("Failed to marshal GraphQL payload", slog.Any("error", err))
		return nil, fmt.Errorf("failed to marshal GraphQL payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphqlURL, bytes.NewBuffer(jsonData))
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

	if resp.StatusCode != http.StatusOK {
		logger.Error("GraphQL request failed",
			slog.Int("status_code", resp.StatusCode),
			slog.String("response", string(body)))
		return nil, fmt.Errorf("GraphQL request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data struct {
			CreateEnterpriseOrganization struct {
				Organization Organization `json:"organization"`
			} `json:"createEnterpriseOrganization"`
		} `json:"data"`
		Errors []struct {
			Message string   `json:"message"`
			Path    []string `json:"path"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		logger.Error("Failed to parse response", slog.Any("error", err))
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for GraphQL errors
	if len(result.Errors) > 0 {
		logger.Error("GraphQL errors returned",
			slog.String("message", result.Errors[0].Message),
			slog.Any("errors", result.Errors))
		return nil, fmt.Errorf("GraphQL errors: %v", result.Errors)
	}

	logger.Info("Successfully created organization",
		slog.String("org", orgName),
		slog.String("user", user),
		slog.Any("response", result))

	org := &result.Data.CreateEnterpriseOrganization.Organization

	// Add the user as admin after org creation (if not already in facilitators list)
	isUserInFacilitators := false
	for _, facilitator := range facilitators {
		if facilitator == user {
			isUserInFacilitators = true
			break
		}
	}

	if !isUserInFacilitators && len(facilitators) > 0 {
		logger.Info("Adding user as organization admin", slog.String("user", user), slog.String("org", org.Login))
		if err := AddOrgMember(ctx, logger, org.Login, user, "admin"); err != nil {
			logger.Error("Failed to add user as admin",
				slog.String("user", user),
				slog.String("org", org.Login),
				slog.Any("error", err))
			// Don't fail the whole operation, just log the error
			logger.Warn("Organization created but user was not added as admin - manual intervention may be required")
		}
	}

	return org, nil
}

// AddOrgMember adds or updates a user's organization membership
// role can be "admin" or "member"
func AddOrgMember(ctx context.Context, logger *slog.Logger, orgName string, username string, role string) error {
	logger.Info("Adding user to organization",
		slog.String("org", orgName),
		slog.String("user", username),
		slog.String("role", role))

	ctx, cancel := context.WithTimeout(ctx, 5000*time.Second)
	defer cancel()

	rt := NewGithubStyleTransport(ctx, logger, config.EnterpriseType)
	client := &http.Client{
		Transport: rt,
	}

	baseURL := ctx.Value(config.BaseURLKey).(string)
	apiURL := fmt.Sprintf("%s/orgs/%s/memberships/%s", baseURL, orgName, username)

	payload := map[string]interface{}{
		"role": role,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		logger.Error("Failed to marshal request payload", slog.Any("error", err))
		return fmt.Errorf("failed to marshal request payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL, bytes.NewBuffer(jsonData))
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read response body", slog.Any("error", err))
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		logger.Error("Failed to add user to organization",
			slog.Int("status_code", resp.StatusCode),
			slog.String("response", string(body)))
		return fmt.Errorf("failed to add user with status %d: %s", resp.StatusCode, string(body))
	}

	var membership struct {
		URL   string `json:"url"`
		State string `json:"state"`
		Role  string `json:"role"`
		User  struct {
			Login string `json:"login"`
		} `json:"user"`
	}

	if err := json.Unmarshal(body, &membership); err != nil {
		logger.Error("Failed to parse response", slog.Any("error", err))
		return fmt.Errorf("failed to parse response: %w", err)
	}

	logger.Info("Successfully added user to organization",
		slog.String("org", orgName),
		slog.String("user", username),
		slog.String("role", membership.Role),
		slog.String("state", membership.State))

	return nil
}

func (enterprise *Enterprise) DeleteOrg(ctx context.Context, logger *slog.Logger, orgLogin string) error {
	logger.Info("Deleting organization", slog.String("org", orgLogin))
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rt := NewGithubStyleTransport(ctx, logger, config.EnterpriseType)

	client := &http.Client{
		Transport: rt,
	}

	baseURL := ctx.Value(config.BaseURLKey).(string)
	graphqlURL := baseURL + "/graphql"

	queryOrg := `
		query($login: String!) {
			organization(login: $login) {
				id
				login
			}
		}
	`

	queryPayload := map[string]interface{}{
		"query": queryOrg,
		"variables": map[string]interface{}{
			"login": orgLogin,
		},
	}

	jsonData, err := json.Marshal(queryPayload)
	if err != nil {
		logger.Error("Failed to marshal GraphQL query payload", slog.Any("error", err))
		return fmt.Errorf("failed to marshal GraphQL query payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphqlURL, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Error("Failed to create query request", slog.Any("error", err))
		return fmt.Errorf("failed to create query request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to execute query request", slog.Any("error", err))
		return fmt.Errorf("failed to execute query request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read query response body", slog.Any("error", err))
		return fmt.Errorf("failed to read query response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("GraphQL query request failed",
			slog.Int("status_code", resp.StatusCode),
			slog.String("response", string(body)))
		return fmt.Errorf("GraphQL query request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var queryResult struct {
		Data struct {
			Organization *Organization `json:"organization"`
		} `json:"data"`
		Errors []struct {
			Message string   `json:"message"`
			Path    []string `json:"path"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &queryResult); err != nil {
		logger.Error("Failed to parse query response", slog.Any("error", err))
		return fmt.Errorf("failed to parse query response: %w", err)
	}

	if len(queryResult.Errors) > 0 {
		logger.Error("GraphQL query errors returned",
			slog.String("message", queryResult.Errors[0].Message),
			slog.Any("errors", queryResult.Errors))
		return fmt.Errorf("GraphQL query errors: %v", queryResult.Errors)
	}

	if queryResult.Data.Organization == nil {
		logger.Error("Organization not found", slog.String("org", orgLogin))
		return fmt.Errorf("organization not found: %s", orgLogin)
	}

	orgID := queryResult.Data.Organization.ID
	logger.Info("Found organization to delete", slog.String("org", orgLogin), slog.String("id", orgID))

	mutation := `
		mutation($enterpriseId: ID!, $organizationId: ID!) {
			removeEnterpriseOrganization(input: {
				enterpriseId: $enterpriseId
				organizationId: $organizationId
			}) {
				clientMutationId
				enterprise {
					id
				}
				organization {
					id
					name
				}
				viewer {
					id
				}
			}
		}
	`

	deletePayload := map[string]interface{}{
		"query": mutation,
		"variables": map[string]interface{}{
			"enterpriseId":   enterprise.ID,
			"organizationId": orgID,
		},
	}

	jsonData, err = json.Marshal(deletePayload)
	if err != nil {
		logger.Error("Failed to marshal GraphQL delete payload", slog.Any("error", err))
		return fmt.Errorf("failed to marshal GraphQL delete payload: %w", err)
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodPost, graphqlURL, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Error("Failed to create delete request", slog.Any("error", err))
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	resp, err = client.Do(req)
	if err != nil {
		logger.Error("Failed to execute delete request", slog.Any("error", err))
		return fmt.Errorf("failed to execute delete request: %w", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read delete response body", slog.Any("error", err))
		return fmt.Errorf("failed to read delete response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("GraphQL delete request failed",
			slog.Int("status_code", resp.StatusCode),
			slog.String("response", string(body)))
		return fmt.Errorf("GraphQL delete request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var deleteResult struct {
		Data struct {
			RemoveEnterpriseOrganization struct {
				ClientMutationID string `json:"clientMutationId"`
			} `json:"removeEnterpriseOrganization"`
		} `json:"data"`
		Errors []struct {
			Message string   `json:"message"`
			Path    []string `json:"path"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &deleteResult); err != nil {
		logger.Error("Failed to parse delete response", slog.Any("error", err))
		return fmt.Errorf("failed to parse delete response: %w", err)
	}

	if len(deleteResult.Errors) > 0 {
		logger.Error("GraphQL delete errors returned",
			slog.String("message", deleteResult.Errors[0].Message),
			slog.Any("errors", deleteResult.Errors))
		return fmt.Errorf("GraphQL delete errors: %v", deleteResult.Errors)
	}

	logger.Info("Successfully deleted organization",
		slog.String("org", orgLogin),
		slog.String("id", orgID))

	return nil
}

// GetOrganization retrieves an organization by name using REST API
// Note: This returns the numeric ID from REST API, not the GraphQL node ID
func GetOrganization(ctx context.Context, logger *slog.Logger, orgName string) (*Organization, error) {
	logger.Info("Getting organization", slog.String("org", orgName))
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	baseURL := ctx.Value(config.BaseURLKey).(string)
	apiURL := fmt.Sprintf("%s/orgs/%s", baseURL, orgName)

	rt := NewGithubStyleTransport(ctx, logger, config.EnterpriseType)
	client := &http.Client{
		Transport: rt,
	}

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
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read response body", slog.Any("error", err))
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("Failed to get organization",
			slog.Int("status_code", resp.StatusCode),
			slog.String("response", string(body)))
		return nil, fmt.Errorf("failed to get organization with status %d: %s", resp.StatusCode, string(body))
	}

	// REST API returns id as int64, which is fine since we only use this for lookups
	var org struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(body, &org); err != nil {
		logger.Error("Failed to parse response", slog.Any("error", err))
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	logger.Info("Successfully retrieved organization",
		slog.String("org", org.Login),
		slog.String("name", org.Name),
		slog.Int64("id", org.ID))

	// Convert to Organization struct (ID will be string representation of the number)
	return &Organization{
		ID:    fmt.Sprintf("%d", org.ID),
		Login: org.Login,
		Name:  org.Name,
	}, nil
}

// InstallAppOnOrg installs a GitHub App on an organization using REST API
func (enterprise *Enterprise) InstallAppOnOrg(ctx context.Context, logger *slog.Logger, orgName string) (*AppInstallation, error) {
	logger.Info("Installing app on organization",
		slog.String("org", orgName))

	//I don't love this but to get the ClientID we need to get an enterprise installation token again. Consider refactoring later.
	ts := auth.NewTokenService(ctx.Value(config.AppIDKey).(string), ctx.Value(config.PrivateKeyKey).(string), ctx.Value(config.BaseURLKey).(string))
	token, err := ts.GetInstallationToken(config.EnterpriseType)

	if err != nil {
		return nil, fmt.Errorf("failed to get installation token: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rt := NewGithubStyleTransport(ctx, logger, config.EnterpriseType)
	client := &http.Client{
		Transport: rt,
	}

	baseURL := ctx.Value(config.BaseURLKey).(string)
	enterpriseSlug := enterprise.Slug
	apiURL := fmt.Sprintf("%s/enterprises/%s/apps/organizations/%s/installations", baseURL, enterpriseSlug, orgName)

	// Prepare request body
	payload := map[string]interface{}{
		"client_id":            token.ClientID,
		"repository_selection": "all",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		logger.Error("Failed to marshal request payload", slog.Any("error", err))
		return nil, fmt.Errorf("failed to marshal request payload: %w", err)
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

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		logger.Error("Failed to install app on organization",
			slog.Int("status_code", resp.StatusCode),
			slog.String("response", string(body)))
		return nil, fmt.Errorf("failed to install app with status %d: %s", resp.StatusCode, string(body))
	}

	var installation AppInstallation
	if err := json.Unmarshal(body, &installation); err != nil {
		logger.Error("Failed to parse response", slog.Any("error", err))
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	logger.Info("Successfully installed app on organization",
		slog.String("org", orgName),
		slog.String("app_id", token.AppID),
		slog.Int64("installation_id", installation.ID))

	return &installation, nil
}
