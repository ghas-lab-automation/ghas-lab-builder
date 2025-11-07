package orgs

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/s-samadi/ghas-lab-builder/internal/config"
	api "github.com/s-samadi/ghas-lab-builder/internal/github"
	"github.com/spf13/cobra"
)

var DeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an organization from a lab environment",
	Long:  "Delete an existing organization with the specified user and lab date",
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

		// Get enterprise information
		enterpriseSlug := ctx.Value(config.EnterpriseSlugKey).(string)
		enterprise, err := api.GetEnterprise(ctx, logger, enterpriseSlug)
		if err != nil {
			logger.Error("Failed to get enterprise info", slog.Any("error", err))
			return fmt.Errorf("failed to get enterprise info: %w", err)
		}

		// Build org name from lab date and user
		orgName := fmt.Sprintf("ghas-labs-%s-%s", labDate, user)

		// Delete organization
		err = enterprise.DeleteOrg(ctx, logger, orgName)
		if err != nil {
			logger.Error("Failed to delete organization",
				slog.String("org", orgName),
				slog.Any("error", err))
			return fmt.Errorf("failed to delete organization: %w", err)
		}

		logger.Info("Successfully deleted organization",
			slog.String("org", orgName),
			slog.String("user", user),
			slog.String("lab_date", labDate))

		return nil
	},
}
