package orgs

import (
	"github.com/spf13/cobra"
)

var (
	labDate string
	user    string
)

var OrgsCmd = &cobra.Command{
	Use:   "orgs",
	Short: "Manage organizations within lab environments",
	Long:  "The 'orgs' command lets you create, delete, and manage organizations within GitHub Advanced Security lab environments.",
}

func init() {
	OrgsCmd.PersistentFlags().StringVar(&labDate, "lab-date", "", "Date string to identify date of the lab (e.g., '2024-06-15') (required)")
	OrgsCmd.MarkPersistentFlagRequired("lab-date")

	OrgsCmd.PersistentFlags().StringVar(&user, "user", "", "User identifier for the organization (required)")
	OrgsCmd.MarkPersistentFlagRequired("user")

	OrgsCmd.AddCommand(CreateCmd)
	OrgsCmd.AddCommand(DeleteCmd)
}
