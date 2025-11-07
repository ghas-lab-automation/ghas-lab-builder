package repo

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/s-samadi/ghas-lab-builder/internal/config"
	reposervice "github.com/s-samadi/ghas-lab-builder/internal/services"
	"github.com/s-samadi/ghas-lab-builder/internal/util"
	"github.com/spf13/cobra"
)

var (
	deleteRepos string
)

func init() {
	DeleteCmd.PersistentFlags().StringVar(&deleteRepos, "repos", "", "Path to file containing repository names to delete (JSON). If empty, all repos in the org will be deleted")
}

var DeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete repositories within a lab environment",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {

		root := cmd
		for root.Parent() != nil {
			root = root.Parent()
		}

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

		var repoNames []string

		if deleteRepos != "" {
			repoConfigs, err := util.LoadFromJsonFile(deleteRepos)
			if err != nil {
				logger.Error("Failed to load repository names",
					slog.String("file", deleteRepos),
					slog.Any("error", err))
				return err
			}

			repoNames = make([]string, len(repoConfigs))
			for i, config := range repoConfigs {

				parts := strings.Split(config.Template, "/")
				if len(parts) == 2 {
					repoNames[i] = parts[1]
				} else {
					repoNames[i] = config.Template
				}
			}
		} else {
			logger.Info("No repos file specified, will delete all repositories in the organization")
			repoNames = nil
		}

		return reposervice.DeleteReposInLabOrg(ctx, logger, repoNames)
	},
}
