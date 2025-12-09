package services

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/s-samadi/ghas-lab-builder/internal/config"
	api "github.com/s-samadi/ghas-lab-builder/internal/github"
	"github.com/s-samadi/ghas-lab-builder/internal/util"
)

// ProvisionResult represents the result of provisioning an organization
type ProvisionResult struct {
	User        string
	OrgName     string
	Status      string
	Error       string
	Repos       []RepoReport
	CompletedAt time.Time
}

func ProvisionOrgResources(workerId int, ctx context.Context, logger *slog.Logger, orgChan chan string, resultsChan chan ProvisionResult, enterprise *api.Enterprise, templateRepos []util.RepoConfig) {

	logger.Info("Worker started", slog.Int("workerId", workerId))

	// Create a new organization for the user
	for user := range orgChan {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			logger.Warn("Worker stopping due to context cancellation", slog.Int("workerId", workerId))
			return
		default:
		}

		// Initialize result tracking
		result := ProvisionResult{
			User:        user,
			Status:      "failed",
			Repos:       []RepoReport{},
			CompletedAt: time.Now(),
		}

		// Call the GraphQL-based CreateOrg function
		organization, err := enterprise.CreateOrg(ctx, logger, user)
		if err != nil {
			logger.Error("Failed to create organization",
				slog.String("user", user),
				slog.Any("error", err))
			result.Error = fmt.Sprintf("Failed to create organization: %v", err)
			resultsChan <- result
			continue
		}
		orgName := organization.Login
		result.OrgName = orgName

		//Install app on organization if app installation provided and not PAT
		if ctx.Value(config.TokenKey) == nil {

			_, err = enterprise.InstallAppOnOrg(ctx, logger, orgName)
			if err != nil {
				logger.Error("Failed to install app on organization",
					slog.String("org", orgName),
					slog.Any("error", err))
				result.Error = fmt.Sprintf("Failed to install app: %v", err)
				resultsChan <- result
				continue
			}
		}

		// Add organization name to context for token scoping (must be after app installation)
		ctx = context.WithValue(ctx, config.OrgKey, orgName)

		// Add the user as admin after app installation (if not already in facilitators list)
		facilitators := ctx.Value(config.FacilitatorsKey).([]string)
		isUserInFacilitators := false
		for _, facilitator := range facilitators {
			if facilitator == user {
				isUserInFacilitators = true
				break
			}
		}

		if !isUserInFacilitators && len(facilitators) > 0 {
			logger.Info("Adding user as organization admin", slog.String("user", user), slog.String("org", orgName))
			if err := api.AddOrgMember(ctx, logger, orgName, user, "admin"); err != nil {
				logger.Error("Failed to add user as admin",
					slog.String("user", user),
					slog.String("org", orgName),
					slog.Any("error", err))
				logger.Warn("Organization created but user was not added as admin - manual intervention may be required")
			}
		}

		logger.Info("Creating repositories in organization", slog.String("org", orgName))

		// Track each repository creation
		repoSuccessCount := 0
		repoFailureCount := 0
		for _, repoConfig := range templateRepos {
			logger.Info("Creating repository",
				slog.String("repo", repoConfig.Template),
				slog.Bool("include_all_branches", repoConfig.IncludeAllBranches))

			repoResult := RepoReport{
				Name:   repoConfig.Template,
				Status: "failed",
			}

			createdRepo, err := organization.CreateRepoFromTemplate(ctx, logger, repoConfig.Template, repoConfig.IncludeAllBranches)
			if err != nil {
				logger.Error("Failed to create repository",
					slog.String("repo", repoConfig.Template),
					slog.Any("error", err))
				repoResult.Error = fmt.Sprintf("%v", err)
				repoFailureCount++
			} else {
				repoResult.Status = "success"
				repoResult.URL = createdRepo.HTMLURL
				repoSuccessCount++
			}
			result.Repos = append(result.Repos, repoResult)
		}

		// Mark as success only if all repositories were created successfully
		if repoFailureCount == 0 {
			result.Status = "success"
		} else {
			result.Status = "partial"
			result.Error = fmt.Sprintf("Failed to create %d out of %d repositories", repoFailureCount, len(templateRepos))
		}
		resultsChan <- result
		logger.Info("Finished creating organization",
			slog.String("org", orgName),
			slog.Int("repo_success", repoSuccessCount),
			slog.Int("repo_failures", repoFailureCount))

	}

	logger.Info("Worker stopped", slog.Int("workerId", workerId))
}

func CreateLabEnvironment(ctx context.Context, logger *slog.Logger, usersFile string, templateReposFile string) error {

	//Get users
	logger.Info("Loading users from file", slog.String("file", usersFile))
	users, err := util.LoadFromFile(usersFile)
	if err != nil {
		return err
	}

	logger.Info("Loaded users", slog.Int("count", len(users)))

	// Get facilitators from context
	facilitators, _ := ctx.Value(config.FacilitatorsKey).([]string)

	// Validate and filter users
	logger.Info("Validating users", slog.Int("count", len(users)))
	userValidation, err := api.ValidateAndFilterUsers(ctx, logger, users)
	if err != nil {
		logger.Error("User validation failed", slog.Any("error", err))
		return fmt.Errorf("user validation failed: %w", err)
	}

	invalidUsers := userValidation.InvalidUsers
	users = userValidation.ValidUsers

	// Validate and filter facilitators
	invalidFacilitators := []string{}
	if len(facilitators) > 0 {
		logger.Info("Validating facilitators", slog.Int("count", len(facilitators)))
		facilitatorValidation, err := api.ValidateAndFilterUsers(ctx, logger, facilitators)
		if err != nil {
			logger.Error("Facilitator validation failed", slog.Any("error", err))
			return fmt.Errorf("facilitator validation failed: %w", err)
		}
		invalidFacilitators = facilitatorValidation.InvalidUsers
		facilitators = facilitatorValidation.ValidUsers
		// Update context with filtered facilitators
		ctx = context.WithValue(ctx, config.FacilitatorsKey, facilitators)
	}

	// Combine users and facilitators for provisioning
	// Use a map to efficiently track unique users
	userSet := make(map[string]bool, len(users)+len(facilitators))

	// Add all users first
	for _, user := range users {
		userSet[user] = true
	}

	// Add facilitators only if not already present
	for _, facilitator := range facilitators {
		userSet[facilitator] = true
	}

	// Convert map to slice
	allUsersToProvision := make([]string, 0, len(userSet))
	for user := range userSet {
		allUsersToProvision = append(allUsersToProvision, user)
	}

	logger.Info("Proceeding with validated users",
		slog.Int("student_count", len(users)),
		slog.Int("facilitator_count", len(facilitators)),
		slog.Int("total_provision_count", len(allUsersToProvision)),
		slog.Int("invalid_user_count", len(invalidUsers)),
		slog.Int("invalid_facilitator_count", len(invalidFacilitators)))

	templateRepos, err := util.LoadFromJsonFile(templateReposFile)
	if err != nil {
		return err
	}

	// Get enterprise slug from context
	enterpriseSlug, ok := ctx.Value(config.EnterpriseSlugKey).(string)
	if !ok {
		logger.Error("Enterprise slug not found in context")
		return fmt.Errorf("enterprise slug not found in context")
	}

	// Get lab date from context
	labDate, ok := ctx.Value(config.LabDateKey).(string)
	if !ok {
		logger.Error("Lab date not found in context")
		return fmt.Errorf("lab date not found in context")
	}

	//Get Enterprise details
	enterprise, err := api.GetEnterprise(ctx, logger, enterpriseSlug)
	if err != nil {
		logger.Error("Failed to get enterprise details", slog.String("slug", enterpriseSlug), slog.Any("error", err))
		return err
	}

	orgChan := make(chan string, len(allUsersToProvision))
	// Update channel size to accommodate all users
	resultsChan := make(chan ProvisionResult, len(allUsersToProvision))

	// Use WaitGroup to track worker goroutines
	var wg sync.WaitGroup

	// Calculate optimal number of workers: max 9 or number of users
	numWorkers := 9
	if len(allUsersToProvision) < numWorkers {
		numWorkers = len(allUsersToProvision)
	}
	logger.Info("Starting workers", slog.Int("worker_count", numWorkers), slog.Int("total_user_count", len(allUsersToProvision)))

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerId int) {
			defer wg.Done()
			ProvisionOrgResources(workerId, ctx, logger, orgChan, resultsChan, enterprise, templateRepos)
		}(i)
	}

	// Send all users (students + facilitators) to the channel
	for _, user := range allUsersToProvision {
		orgChan <- user
	}
	// Close orgChan immediately after sending all work
	close(orgChan)

	// Close resultsChan once all workers are done
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results for report
	var results []ProvisionResult
	resultCount := 0
	successCount := 0
	failureCount := 0

	for {
		select {
		case res, ok := <-resultsChan:
			if !ok {
				// Channel closed, all workers finished
				logger.Info("All provisioning complete",
					slog.Int("total", len(allUsersToProvision)),
					slog.Int("success", successCount),
					slog.Int("failed", failureCount))

				// Generate report
				report := &LabReport{
					GeneratedAt:         time.Now(),
					LabDate:             labDate,
					EnterpriseSlug:      enterpriseSlug,
					TotalUsers:          len(allUsersToProvision),
					SuccessCount:        successCount,
					FailureCount:        failureCount,
					TemplateRepos:       getTemplateNames(templateRepos),
					Facilitators:        facilitators,
					InvalidUsers:        invalidUsers,
					InvalidFacilitators: invalidFacilitators,
					Organizations:       make([]OrgReport, 0, len(results)),
				}

				for _, res := range results {
					orgReport := OrgReport{
						User:         res.User,
						OrgName:      res.OrgName,
						Status:       res.Status,
						Error:        res.Error,
						Repositories: res.Repos,
						CreatedAt:    res.CompletedAt,
					}
					report.Organizations = append(report.Organizations, orgReport)
				}

				// Generate report files
				if err := GenerateReportFiles(report, "reports"); err != nil {
					logger.Error("Failed to generate report files", slog.Any("error", err))
				}

				if resultCount == len(allUsersToProvision) {
					if failureCount > 0 {
						logger.Info("Some organizations or repositories failed to be created",
							slog.Int("failed_count", failureCount))
					} else {
						logger.Info("All organizations and repositories created successfully")
						return nil
					}

				}
				logger.Error("Workers finished but not all users processed",
					slog.Int("expected", len(allUsersToProvision)),
					slog.Int("processed", resultCount))
				return ctx.Err()
			}

			// Track results
			results = append(results, res)
			resultCount++

			if res.Status == "success" {
				successCount++
				logger.Info("Created organization", slog.String("org", res.OrgName))
			} else {
				failureCount++
				logger.Error("Failed to create organization",
					slog.String("org", res.OrgName),
					slog.String("error", res.Error))
			}

		case <-ctx.Done():
			logger.Error("Timeout reached while creating lab environment")
			return ctx.Err()
		}
	}
}

// Helper function to extract template names for the report
func getTemplateNames(configs []util.RepoConfig) []string {
	names := make([]string, len(configs))
	for i, config := range configs {
		names[i] = config.Template
	}
	return names
}

func DestroyOrgResources(workerId int, ctx context.Context, logger *slog.Logger, userChan chan string, resultsChan chan string, enterprise *api.Enterprise, labDate string) {
	logger.Info("Destroy worker started", slog.Int("workerId", workerId))

	for user := range userChan {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			logger.Warn("Destroy worker stopping due to context cancellation", slog.Int("workerId", workerId))
			return
		default:
		}

		orgName := "ghas-labs-" + labDate + "-" + user
		logger.Info("Deleting organization", slog.String("org", orgName), slog.String("user", user))

		if err := api.DeleteOrg(ctx, logger, orgName); err != nil {
			logger.Error("Failed to delete organization",
				slog.String("user", user),
				slog.String("org", orgName),
				slog.Any("error", err))
			// Still send result to avoid blocking
			resultsChan <- "failed:" + orgName
			continue
		}

		resultsChan <- orgName
		logger.Info("Finished deleting organization", slog.String("org", orgName))
	}

	logger.Info("Destroy worker stopped", slog.Int("workerId", workerId))
}

func DestroyLabEnvironment(ctx context.Context, logger *slog.Logger, labDate string, usersFile string) error {

	startTime := time.Now()

	// Get users
	logger.Info("Loading users from file", slog.String("file", usersFile))
	users, err := util.LoadFromFile(usersFile)
	if err != nil {
		return err
	}

	logger.Info("Loaded users", slog.Int("count", len(users)))

	// Get enterprise slug from context
	enterpriseSlug, ok := ctx.Value(config.EnterpriseSlugKey).(string)
	if !ok {
		logger.Error("Enterprise slug not found in context")
		return fmt.Errorf("enterprise slug not found in context")
	}

	// Get facilitators from context
	facilitators, _ := ctx.Value(config.FacilitatorsKey).([]string)

	// Validate and filter users
	logger.Info("Validating users", slog.Int("count", len(users)))
	userValidation, err := api.ValidateAndFilterUsers(ctx, logger, users)
	if err != nil {
		logger.Error("User validation failed", slog.Any("error", err))
		return fmt.Errorf("user validation failed: %w", err)
	}

	invalidUsers := userValidation.InvalidUsers
	users = userValidation.ValidUsers

	// Validate and filter facilitators
	invalidFacilitators := []string{}
	if len(facilitators) > 0 {
		logger.Info("Validating facilitators", slog.Int("count", len(facilitators)))
		facilitatorValidation, err := api.ValidateAndFilterUsers(ctx, logger, facilitators)
		if err != nil {
			logger.Error("Facilitator validation failed", slog.Any("error", err))
			return fmt.Errorf("facilitator validation failed: %w", err)
		}
		invalidFacilitators = facilitatorValidation.InvalidUsers
		facilitators = facilitatorValidation.ValidUsers
	}

	// Combine users and facilitators for deletion
	// Use a map to efficiently track unique users
	userSet := make(map[string]bool, len(users)+len(facilitators))

	for _, user := range users {
		userSet[user] = true
	}

	for _, facilitator := range facilitators {
		userSet[facilitator] = true
	}

	allUsersToDelete := make([]string, 0, len(userSet))
	for user := range userSet {
		allUsersToDelete = append(allUsersToDelete, user)
	}

	logger.Info("Proceeding with validated users for deletion",
		slog.Int("student_count", len(users)),
		slog.Int("facilitator_count", len(facilitators)),
		slog.Int("total_delete_count", len(allUsersToDelete)),
		slog.Int("invalid_user_count", len(invalidUsers)),
		slog.Int("invalid_facilitator_count", len(invalidFacilitators)))

	// Get Enterprise details
	enterprise, err := api.GetEnterprise(ctx, logger, enterpriseSlug)
	if err != nil {
		logger.Error("Failed to get enterprise details", slog.String("slug", enterpriseSlug), slog.Any("error", err))
		return err
	}

	// Initialize delete report
	deleteReport := &DeleteLabReport{
		GeneratedAt:         time.Now(),
		LabDate:             labDate,
		TotalUsers:          len(users),
		SuccessCount:        0,
		FailureCount:        0,
		Organizations:       make([]DeleteOrgReport, 0),
		Facilitators:        facilitators,
		InvalidUsers:        invalidUsers,
		InvalidFacilitators: invalidFacilitators,
	}

	userChan := make(chan string, len(allUsersToDelete))
	resultsChan := make(chan DeleteOrgReport, len(allUsersToDelete))

	// Use WaitGroup to track worker goroutines
	var wg sync.WaitGroup

	// Calculate optimal number of workers: min(9, number of users)
	numWorkers := 9
	if len(allUsersToDelete) < numWorkers {
		numWorkers = len(allUsersToDelete)
	}
	logger.Info("Starting destroy workers", slog.Int("worker_count", numWorkers), slog.Int("total_user_count", len(allUsersToDelete)))

	// Create worker goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerId int) {
			defer wg.Done()
			DestroyOrgResourcesWithReport(workerId, ctx, logger, userChan, resultsChan, enterprise, labDate)
		}(i)
	}

	// Send all users (students + facilitators) to the channel
	for _, user := range allUsersToDelete {
		userChan <- user
	}
	// Close userChan immediately after sending all work
	close(userChan)

	// Close resultsChan once all workers are done
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	resultCount := 0

	for {
		select {
		case res, ok := <-resultsChan:
			if !ok {
				// Channel closed, all workers finished
				logger.Info("Finished destroying lab environment",
					slog.String("lab_date", labDate),
					slog.Int("total", len(allUsersToDelete)),
					slog.Int("processed", resultCount),
					slog.Int("successful", deleteReport.SuccessCount),
					slog.Int("failed", deleteReport.FailureCount),
					slog.Duration("duration", time.Since(startTime)))

				// Generate report
				if err := GenerateDeleteReportFiles(deleteReport, "reports"); err != nil {
					logger.Error("Failed to generate deletion report", slog.Any("error", err))
				}

				if deleteReport.FailureCount > 0 {
					return fmt.Errorf("failed to delete %d organization(s)", deleteReport.FailureCount)
				}
				return nil
			}

			resultCount++
			deleteReport.Organizations = append(deleteReport.Organizations, res)

			if res.Status == "success" {
				deleteReport.SuccessCount++
			} else {
				deleteReport.FailureCount++
			}

		case <-ctx.Done():
			logger.Error("Timeout reached while destroying lab environment")

			// Generate report even on timeout
			if err := GenerateDeleteReportFiles(deleteReport, "reports"); err != nil {
				logger.Error("Failed to generate deletion report", slog.Any("error", err))
			}

			return ctx.Err()
		}
	}
}

func DestroyOrgResourcesWithReport(workerId int, ctx context.Context, logger *slog.Logger, userChan chan string, resultsChan chan DeleteOrgReport, enterprise *api.Enterprise, labDate string) {
	logger.Info("Destroy worker started", slog.Int("workerId", workerId))

	for user := range userChan {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			logger.Warn("Destroy worker stopping due to context cancellation", slog.Int("workerId", workerId))
			return
		default:
		}

		orgName := "ghas-labs-" + labDate + "-" + user
		logger.Info("Deleting organization", slog.String("org", orgName), slog.String("user", user))

		deleteTime := time.Now()
		orgReport := DeleteOrgReport{
			User:      user,
			OrgName:   orgName,
			DeletedAt: deleteTime,
		}

		// Call the GraphQL-based DeleteOrg function
		if err := api.DeleteOrg(ctx, logger, orgName); err != nil {
			logger.Error("Failed to delete organization",
				slog.String("user", user),
				slog.String("org", orgName),
				slog.Any("error", err))

			orgReport.Status = "failed"
			orgReport.Error = err.Error()
			resultsChan <- orgReport
			continue
		}

		orgReport.Status = "success"
		resultsChan <- orgReport
		logger.Info("Finished deleting organization", slog.String("org", orgName))
	}

	logger.Info("Destroy worker stopped", slog.Int("workerId", workerId))
}
