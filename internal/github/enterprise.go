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

	"github.com/s-samadi/ghas-lab-builder/internal/config"
)

// GetEnterprise retrieves enterprise information using the enterprise slug via GraphQL
func GetEnterprise(ctx context.Context, logger *slog.Logger, enterpriseSlug string) (*Enterprise, error) {
	logger.Info("Fetching enterprise", slog.String("slug", enterpriseSlug))

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	rt := NewGithubStyleTransport(ctx, logger, config.EnterpriseType)
	client := &http.Client{
		Transport: rt,
	}

	baseURL := ctx.Value(config.BaseURLKey).(string)
	graphqlURL := baseURL + "/graphql"

	// GraphQL query to fetch enterprise by slug
	query := `
		query($slug: String!) {
			enterprise(slug: $slug) {
				id,
				billingEmail,
				slug
			}
		}
	`

	// Build the GraphQL request payload
	payload := map[string]interface{}{
		"query": query,
		"variables": map[string]interface{}{
			"slug": enterpriseSlug,
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
			Enterprise Enterprise `json:"enterprise"`
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
		logger.Error("GraphQL errors",
			slog.String("message", result.Errors[0].Message),
			slog.Any("errors", result.Errors))
		return nil, fmt.Errorf("GraphQL error: %s", result.Errors[0].Message)
	}

	if result.Data.Enterprise.ID == "" {
		logger.Error("Enterprise not found", slog.String("slug", enterpriseSlug))
		return nil, fmt.Errorf("enterprise not found: %s", enterpriseSlug)
	}

	logger.Info("Enterprise retrieved successfully",
		slog.String("id", result.Data.Enterprise.ID),
		slog.String("slug", result.Data.Enterprise.Slug),
		slog.String("billing email", result.Data.Enterprise.BillingEmail))

	return &result.Data.Enterprise, nil
}
