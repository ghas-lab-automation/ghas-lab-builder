package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/s-samadi/ghas-lab-builder/cmd/lab"
	"github.com/s-samadi/ghas-lab-builder/cmd/orgs"
	"github.com/s-samadi/ghas-lab-builder/cmd/repo"
	"github.com/s-samadi/ghas-lab-builder/internal/config"
	"github.com/s-samadi/ghas-lab-builder/internal/util"
	"github.com/spf13/cobra"
)

var (
	appId          string
	privateKey     string
	token          string
	baseURL        string
	enterpriseSlug string
)

var rootCmd = &cobra.Command{
	Use:   "ghas-lab-builder",
	Short: "Builds GitHub Advanced Security Lab environments(orgs, repos, users)",
	Long: `ghas-lab-builder is a CLI tool that helps you set up GitHub Advanced Security Lab environments by 
          automating the creation of organizations, repositories, and addings  users required for hands-on labs.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Validate that either token OR (app-id + private-key) is provided, but not both
		hasToken := token != ""
		hasAppCreds := appId != "" || privateKey != ""

		if !hasToken && !hasAppCreds {
			return fmt.Errorf("authentication required: provide either --token OR both --app-id and --private-key")
		}

		if hasToken && hasAppCreds {
			return fmt.Errorf("conflicting authentication methods: provide either --token OR (--app-id and --private-key), not both")
		}

		// If using app credentials, both app-id and private-key must be provided
		if hasAppCreds {
			if appId == "" {
				return fmt.Errorf("--app-id is required when using GitHub App authentication")
			}
			if privateKey == "" {
				return fmt.Errorf("--private-key is required when using GitHub App authentication")
			}
		}

		// Set default base URL if not provided
		if baseURL == "" {
			baseURL = config.DefaultBaseURL
		}

		// Generate log file path automatically
		logFilePath := util.GenerateLogFileName("ghas-lab-builder")

		// Initialize logger with automatic log file
		loggerConfig := util.LoggerConfig{
			LogFilePath: logFilePath,
			LogLevel:    slog.LevelInfo,
		}
		logger, closer, err := util.NewLogger(loggerConfig)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}

		// Store closer in context for cleanup
		if closer != nil {
			cmd.SetContext(context.WithValue(cmd.Context(), "logCloser", closer))
		}

		// Store authentication information in context
		ctx := cmd.Context()
		ctx = context.WithValue(ctx, config.LoggerKey, logger)

		if token != "" {
			// Using PAT authentication
			ctx = context.WithValue(ctx, config.TokenKey, token)
		} else {
			// Using GitHub App authentication
			ctx = context.WithValue(ctx, config.AppIDKey, appId)
			ctx = context.WithValue(ctx, config.PrivateKeyKey, privateKey)
		}

		ctx = context.WithValue(ctx, config.BaseURLKey, baseURL)
		ctx = context.WithValue(ctx, config.EnterpriseSlugKey, enterpriseSlug)

		logger.Info("Logging initialized", slog.String("log_file", logFilePath))

		cmd.SetContext(ctx)
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		// Cleanup: close log file if it was opened
		if closer, ok := cmd.Context().Value("logCloser").(io.Closer); ok && closer != nil {
			return closer.Close()
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// GitHub App authentication flags
	rootCmd.PersistentFlags().StringVar(&appId, "app-id", "", "GitHub App ID (required if not using --token)")
	rootCmd.PersistentFlags().StringVar(&privateKey, "private-key", "", "GitHub App private key PEM content (required if not using --token)")

	// PAT authentication flag
	rootCmd.PersistentFlags().StringVar(&token, "token", "", "GitHub Personal Access Token (required if not using GitHub App authentication)")

	// Common flags
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "", "GitHub API base URL")
	rootCmd.PersistentFlags().StringVar(&enterpriseSlug, "enterprise-slug", "", "GitHub Enterprise slug")
	rootCmd.MarkPersistentFlagRequired("enterprise-slug")

	if baseURL == "" {
		baseURL = config.DefaultBaseURL
	}

	rootCmd.AddCommand(lab.LabCmd)
	rootCmd.AddCommand(repo.RepoCmd)
	rootCmd.AddCommand(orgs.OrgsCmd)
}
