package repo

import (
	"context"
	"log/slog"
	"os"

	"github.com/s-samadi/ghas-lab-builder/internal/config"
	reposervice "github.com/s-samadi/ghas-lab-builder/internal/services"
	"github.com/spf13/cobra"
)

var (
	repos string
)

func init() {
	CreateCmd.PersistentFlags().StringVar(&repos, "repos", "", "Path to template repositories file (JSON) (required)")
	CreateCmd.MarkPersistentFlagRequired("repos")
}

var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create repositories within a lab environment",
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

		ctx := cmd.Context()

		ctx = context.WithValue(ctx, config.OrgKey, org)

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

		return reposervice.CreateReposInLabOrg(ctx, logger, repos)
	},
}
