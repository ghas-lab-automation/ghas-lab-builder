package orgs

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/s-samadi/ghas-lab-builder/internal/config"
	api "github.com/s-samadi/ghas-lab-builder/internal/github"
	"github.com/spf13/cobra"
)

var (
	facilitators string
)

func init() {
	CreateCmd.PersistentFlags().StringVar(&facilitators, "facilitators", "", "Lab facilitators usernames, comma-separated (required)")
	CreateCmd.MarkPersistentFlagRequired("facilitators")
}

var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an organization within a lab environment",
	Long:  "Create a new organization with the specified user and lab date, and install the GitHub App on it",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Traverse up to find and call the root command's PersistentPreRunE
		root := cmd
		for root.Parent() != nil {
			root = root.Parent()
		}

		// Call root's PersistentPreRunE if it exists
		if root.PersistentPreRunE != nil {
			if err := root.PersistentPreRunE(cmd, args); err != nil {
				return err
			}
		}

		// Add org-specific context values
		ctx := cmd.Context()
		ctx = context.WithValue(ctx, config.EnterpriseSlugKey, cmd.Flags().Lookup("enterprise-slug").Value.String())
		ctx = context.WithValue(ctx, config.FacilitatorsKey, strings.Split(facilitators, ","))
		ctx = context.WithValue(ctx, config.LabDateKey, labDate)

		cmd.SetContext(ctx)
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		// Get logger from context (initialized in root command)
		logger, ok := ctx.Value(config.LoggerKey).(*slog.Logger)
		if !ok || logger == nil {
			// Fallback to default logger if not found
			logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
		}

		// Get facilitators from context
		facilitators := ctx.Value(config.FacilitatorsKey).([]string)

		// Validate the user
		logger.Info("Validating user", slog.String("user", user))
		userValidation, err := api.ValidateAndFilterUsers(ctx, logger, []string{user})
		if err != nil {
			logger.Error("User validation failed", slog.Any("error", err))
			return fmt.Errorf("user validation failed: %w", err)
		}
		if len(userValidation.ValidUsers) == 0 {
			return fmt.Errorf("user '%s' not found in GitHub", user)
		}

		// Validate and filter facilitators
		if len(facilitators) > 0 {
			logger.Info("Validating facilitators", slog.Int("count", len(facilitators)))
			facilitatorValidation, err := api.ValidateAndFilterUsers(ctx, logger, facilitators)
			if err != nil {
				logger.Error("Facilitator validation failed", slog.Any("error", err))
				return fmt.Errorf("facilitator validation failed: %w", err)
			}

			if len(facilitatorValidation.InvalidUsers) > 0 {
				logger.Warn("Some facilitators are invalid and will be skipped",
					slog.Any("invalid_facilitators", facilitatorValidation.InvalidUsers))
			}

			facilitators = facilitatorValidation.ValidUsers
			// Update context with filtered facilitators
			ctx = context.WithValue(ctx, config.FacilitatorsKey, facilitators)
			logger.Info("Proceeding with validated facilitators", slog.Int("count", len(facilitators)))
		}

		// Get enterprise information
		enterpriseSlug := ctx.Value(config.EnterpriseSlugKey).(string)
		enterprise, err := api.GetEnterprise(ctx, logger, enterpriseSlug)
		if err != nil {
			logger.Error("Failed to get enterprise info", slog.Any("error", err))
			return fmt.Errorf("failed to get enterprise info: %w", err)
		}

		// Create organization and add user
		org, err := enterprise.CreateOrg(ctx, logger, user)
		if err != nil {
			logger.Error("Failed to create organization", slog.Any("error", err))
			return fmt.Errorf("failed to create organization: %w", err)
		}

		logger.Info("Successfully created organization",
			slog.String("org", org.Login),
			slog.String("user", user),
			slog.String("lab_date", labDate))

		// Install app on the organization
		_, err = enterprise.InstallAppOnOrg(ctx, logger, org.Login)
		if err != nil {
			logger.Error("Failed to install app on organization",
				slog.String("org", org.Login),
				slog.Any("error", err))
			return fmt.Errorf("failed to install app on organization: %w", err)
		}

		logger.Info("Successfully installed app on organization",
			slog.String("org", org.Login))

		return nil
	},
}
