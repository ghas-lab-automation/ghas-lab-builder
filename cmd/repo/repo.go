package repo

import (
	"github.com/spf13/cobra"
)

var (
	org string
)

var RepoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage repositories within lab environments",
	Long:  "The 'repo' command lets you create, delete, and inspect repositories within GitHub Advanced Security lab environments.",
}

func init() {
	RepoCmd.AddCommand(CreateCmd)
	RepoCmd.AddCommand(DeleteCmd)

	RepoCmd.PersistentFlags().StringVar(&org, "org", "", "Organization name for the lab repositories (required)")
	RepoCmd.MarkPersistentFlagRequired("org")
}
