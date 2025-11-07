package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/s-samadi/ghas-lab-builder/internal/config"
	api "github.com/s-samadi/ghas-lab-builder/internal/github"
	util "github.com/s-samadi/ghas-lab-builder/internal/util"
)

// CreateReposInLabOrg creates repositories from templates in an existing lab organization
func CreateReposInLabOrg(ctx context.Context, logger *slog.Logger, templateReposFile string) error {
	logger.Info("Starting repository creation in lab organization")

	// Get organization name from context
	orgName, ok := ctx.Value(config.OrgKey).(string)
	if !ok || orgName == "" {
		return fmt.Errorf("organization name not found in context")
	}

	// Load template repositories from file
	templateRepos, err := util.LoadFromJsonFile(templateReposFile)
	if err != nil {
		logger.Error("Failed to load template repositories",
			slog.String("file", templateReposFile),
			slog.Any("error", err))
		return fmt.Errorf("failed to load template repositories: %w", err)
	}

	logger.Info("Loaded template repositories",
		slog.Int("count", len(templateRepos)),
		slog.String("org", orgName))

	// Get the organization
	organization, err := api.GetOrganization(ctx, logger, orgName)
	if err != nil {
		logger.Error("Failed to get organization",
			slog.String("org", orgName),
			slog.Any("error", err))
		return fmt.Errorf("failed to get organization %s: %w", orgName, err)
	}

	logger.Info("Found organization", slog.String("org", organization.Login))

	// Create repositories from templates
	successCount := 0
	for _, repoConfig := range templateRepos {
		logger.Info("Creating repository from template",
			slog.String("template", repoConfig.Template),
			slog.Bool("include_all_branches", repoConfig.IncludeAllBranches),
			slog.String("org", orgName))

		_, err := organization.CreateRepoFromTemplate(ctx, logger, repoConfig.Template, repoConfig.IncludeAllBranches)
		if err != nil {
			logger.Error("Failed to create repository",
				slog.String("repo", repoConfig.Template),
				slog.String("org", orgName),
				slog.Any("error", err))
			// Continue with other repos even if one fails
			continue
		}

		successCount++
		logger.Info("Successfully created repository",
			slog.String("template", repoConfig.Template),
			slog.String("org", orgName))
	}

	logger.Info("Completed repository creation",
		slog.Int("success_count", successCount),
		slog.Int("total_repos", len(templateRepos)),
		slog.String("org", orgName))

	if successCount == 0 && len(templateRepos) > 0 {
		return fmt.Errorf("failed to create any repositories")
	}

	return nil
}

// DeleteReposInLabOrg deletes repositories in a lab organization
// If repoNames is nil or empty, all repositories in the organization will be deleted
func DeleteReposInLabOrg(ctx context.Context, logger *slog.Logger, repoNames []string) error {
	logger.Info("Starting repository deletion in lab organization")

	// Get organization name from context
	orgName, ok := ctx.Value(config.OrgKey).(string)
	if !ok || orgName == "" {
		return fmt.Errorf("organization name not found in context")
	}

	// Get the organization
	organization, err := api.GetOrganization(ctx, logger, orgName)
	if err != nil {
		logger.Error("Failed to get organization",
			slog.String("org", orgName),
			slog.Any("error", err))
		return fmt.Errorf("failed to get organization %s: %w", orgName, err)
	}

	logger.Info("Found organization", slog.String("org", organization.Login))

	// If no specific repos provided, get all repos in the org
	if len(repoNames) == 0 {
		logger.Info("Fetching all repositories in organization", slog.String("org", orgName))
		repoNames, err = organization.ListRepositories(ctx, logger)
		if err != nil {
			logger.Error("Failed to list repositories",
				slog.String("org", orgName),
				slog.Any("error", err))
			return fmt.Errorf("failed to list repositories: %w", err)
		}
		logger.Info("Found repositories to delete",
			slog.Int("count", len(repoNames)),
			slog.String("org", orgName))
	}

	if len(repoNames) == 0 {
		logger.Info("No repositories to delete", slog.String("org", orgName))
		return nil
	}

	logger.Info("Deleting repositories",
		slog.Int("count", len(repoNames)),
		slog.String("org", orgName))

	// Delete repositories
	successCount := 0
	for _, repoName := range repoNames {
		logger.Info("Deleting repository",
			slog.String("repo", repoName),
			slog.String("org", orgName))

		err := organization.DeleteRepository(ctx, logger, repoName)
		if err != nil {
			logger.Error("Failed to delete repository",
				slog.String("repo", repoName),
				slog.String("org", orgName),
				slog.Any("error", err))
			// Continue with other repos even if one fails
			continue
		}

		successCount++
		logger.Info("Successfully deleted repository",
			slog.String("repo", repoName),
			slog.String("org", orgName))
	}

	logger.Info("Completed repository deletion",
		slog.Int("success_count", successCount),
		slog.Int("total_repos", len(repoNames)),
		slog.String("org", orgName))

	if successCount == 0 && len(repoNames) > 0 {
		return fmt.Errorf("failed to delete any repositories")
	}

	return nil
}
