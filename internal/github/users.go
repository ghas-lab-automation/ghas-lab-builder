package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/s-samadi/ghas-lab-builder/internal/config"
)

// UserValidationResult contains the results of user validation
type UserValidationResult struct {
	ValidUsers   []string
	InvalidUsers []string
}

// ValidateAndFilterUsers checks if all provided usernames exist in GitHub Enterprise
// Returns a UserValidationResult with valid and invalid user lists
func ValidateAndFilterUsers(ctx context.Context, logger *slog.Logger, usernames []string) (*UserValidationResult, error) {
	if len(usernames) == 0 {
		return &UserValidationResult{
			ValidUsers:   []string{},
			InvalidUsers: []string{},
		}, nil
	}

	logger.Info("Validating users", slog.Int("count", len(usernames)))

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Use the existing roundtripper infrastructure
	rt := NewGithubStyleTransport(ctx, logger, config.EnterpriseType)
	client := &http.Client{
		Transport: rt,
	}

	baseURL := ctx.Value(config.BaseURLKey).(string)

	// Use channels and goroutines for concurrent validation
	type validationResult struct {
		username string
		valid    bool
		err      error
	}

	resultChan := make(chan validationResult, len(usernames))
	var wg sync.WaitGroup

	// Validate users concurrently (max 10 at a time to avoid rate limits)
	semaphore := make(chan struct{}, 10)

	for _, username := range usernames {
		wg.Add(1)
		go func(user string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Check if context is cancelled
			select {
			case <-ctx.Done():
				resultChan <- validationResult{username: user, valid: false, err: ctx.Err()}
				return
			default:
			}

			userURL := fmt.Sprintf("%s/users/%s", baseURL, user)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, userURL, nil)
			if err != nil {
				resultChan <- validationResult{username: user, valid: false, err: err}
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				resultChan <- validationResult{username: user, valid: false, err: err}
				return
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				logger.Warn("User not found - will be skipped", slog.String("username", user))
				resultChan <- validationResult{username: user, valid: false, err: nil}
			} else if resp.StatusCode != http.StatusOK {
				logger.Warn("Unexpected status for user - will be skipped",
					slog.String("username", user),
					slog.Int("status", resp.StatusCode))
				resultChan <- validationResult{username: user, valid: false, err: fmt.Errorf("unexpected status: %d", resp.StatusCode)}
			} else {
				logger.Info("User validated", slog.String("username", user))
				resultChan <- validationResult{username: user, valid: true, err: nil}
			}
		}(username)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results - preserve original order
	validationMap := make(map[string]bool)
	invalidUsers := []string{}

	for result := range resultChan {
		if result.valid {
			validationMap[result.username] = true
		} else {
			invalidUsers = append(invalidUsers, result.username)
		}
	}

	// Build valid users list maintaining original order
	validUsers := make([]string, 0, len(usernames))
	for _, username := range usernames {
		if validationMap[username] {
			validUsers = append(validUsers, username)
		}
	}

	if len(invalidUsers) > 0 {
		logger.Warn("Invalid users found and removed",
			slog.Any("invalid_users", invalidUsers),
			slog.Int("invalid_count", len(invalidUsers)),
			slog.Int("valid_count", len(validUsers)),
			slog.Int("total_count", len(usernames)))
	}

	if len(validUsers) == 0 {
		return nil, fmt.Errorf("no valid users found after validation")
	}

	logger.Info("User validation complete",
		slog.Int("valid_count", len(validUsers)),
		slog.Int("invalid_count", len(invalidUsers)))

	return &UserValidationResult{
		ValidUsers:   validUsers,
		InvalidUsers: invalidUsers,
	}, nil
}
