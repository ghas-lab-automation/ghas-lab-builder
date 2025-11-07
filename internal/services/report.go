package services

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LabReport represents the complete lab environment creation report
type LabReport struct {
	GeneratedAt         time.Time   `json:"generated_at"`
	LabDate             string      `json:"lab_date"`
	EnterpriseSlug      string      `json:"enterprise_slug"`
	TotalUsers          int         `json:"total_users"`
	SuccessCount        int         `json:"success_count"`
	FailureCount        int         `json:"failure_count"`
	Organizations       []OrgReport `json:"organizations"`
	TemplateRepos       []string    `json:"template_repos"`
	Facilitators        []string    `json:"facilitators,omitempty"`
	InvalidUsers        []string    `json:"invalid_users,omitempty"`
	InvalidFacilitators []string    `json:"invalid_facilitators,omitempty"`
}

// OrgReport represents the details of a single organization
type OrgReport struct {
	User         string       `json:"user"`
	OrgName      string       `json:"org_name"`
	Status       string       `json:"status"`
	Error        string       `json:"error,omitempty"`
	Repositories []RepoReport `json:"repositories"`
	CreatedAt    time.Time    `json:"created_at"`
}

// RepoReport represents the details of a repository
type RepoReport struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	URL    string `json:"url,omitempty"`
}

// DeleteLabReport represents the complete lab environment deletion report
type DeleteLabReport struct {
	GeneratedAt    time.Time         `json:"generated_at"`
	LabDate        string            `json:"lab_date"`
	EnterpriseSlug string            `json:"enterprise_slug"`
	TotalUsers     int               `json:"total_users"`
	SuccessCount   int               `json:"success_count"`
	FailureCount   int               `json:"failure_count"`
	Organizations  []DeleteOrgReport `json:"organizations"`
	Facilitators   []string          `json:"facilitators,omitempty"`
}

// DeleteOrgReport represents the deletion details of a single organization
type DeleteOrgReport struct {
	User      string    `json:"user"`
	OrgName   string    `json:"org_name"`
	Status    string    `json:"status"` // "success" or "failed"
	Error     string    `json:"error,omitempty"`
	DeletedAt time.Time `json:"deleted_at"`
}

// GenerateReportFiles generates Markdown report and GitHub Actions summary
func GenerateReportFiles(report *LabReport, outputDir string) error {
	if outputDir == "" {
		outputDir = "."
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("lab-report-%s-%s.md", report.LabDate, timestamp)
	mdPath := filepath.Join(outputDir, filename)

	// Generate Markdown report
	if err := generateMarkdownReport(report, mdPath); err != nil {
		return err
	}

	// Generate GitHub Actions Step Summary if running in Actions
	if err := generateGitHubStepSummary(report); err != nil {
		// Don't fail if we can't write to step summary
		fmt.Fprintf(os.Stderr, "Warning: Failed to write GitHub step summary: %v\n", err)
	}

	fmt.Printf("\n✅ Report generated successfully:\n")
	fmt.Printf("  📝 Markdown: %s\n", mdPath)

	return nil
}

// generateGitHubStepSummary writes a summary to GitHub Actions UI
func generateGitHubStepSummary(report *LabReport) error {
	stepSummaryPath := os.Getenv("GITHUB_STEP_SUMMARY")
	if stepSummaryPath == "" {
		// Not running in GitHub Actions, skip
		return nil
	}

	file, err := os.OpenFile(stepSummaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write beautiful markdown summary
	fmt.Fprintf(file, "# 🧪 Lab Environment Report\n\n")

	// Summary badges/stats
	successRate := float64(report.SuccessCount) / float64(report.TotalUsers) * 100
	emoji := "✅"
	if successRate < 100 {
		emoji = "⚠️"
	}
	if successRate < 50 {
		emoji = "❌"
	}

	fmt.Fprintf(file, "> %s **Lab Date:** `%s` | **Enterprise:** `%s`\n\n", emoji, report.LabDate, report.EnterpriseSlug)

	// Stats table
	fmt.Fprintf(file, "## 📊 Summary\n\n")
	fmt.Fprintf(file, "| Metric | Count | Percentage |\n")
	fmt.Fprintf(file, "|--------|------:|-----------:|\n")
	fmt.Fprintf(file, "| **Total Users** | %d | 100%% |\n", report.TotalUsers)
	fmt.Fprintf(file, "| ✅ **Successful** | %d | %.1f%% |\n", report.SuccessCount, successRate)
	fmt.Fprintf(file, "| ❌ **Failed** | %d | %.1f%% |\n", report.FailureCount,
		float64(report.FailureCount)/float64(report.TotalUsers)*100)
	fmt.Fprintf(file, "\n")

	// Invalid users warning
	if len(report.InvalidUsers) > 0 || len(report.InvalidFacilitators) > 0 {
		fmt.Fprintf(file, "## ⚠️ Invalid Users Skipped\n\n")
		if len(report.InvalidUsers) > 0 {
			fmt.Fprintf(file, "**Invalid Users (%d):** ", len(report.InvalidUsers))
			for i, u := range report.InvalidUsers {
				if i > 0 {
					fmt.Fprintf(file, ", ")
				}
				fmt.Fprintf(file, "`@%s`", u)
			}
			fmt.Fprintf(file, "\n\n")
		}
		if len(report.InvalidFacilitators) > 0 {
			fmt.Fprintf(file, "**Invalid Facilitators (%d):** ", len(report.InvalidFacilitators))
			for i, f := range report.InvalidFacilitators {
				if i > 0 {
					fmt.Fprintf(file, ", ")
				}
				fmt.Fprintf(file, "`@%s`", f)
			}
			fmt.Fprintf(file, "\n\n")
		}
	}

	// Facilitators
	if len(report.Facilitators) > 0 {
		fmt.Fprintf(file, "**👥 Facilitators:** ")
		for i, f := range report.Facilitators {
			if i > 0 {
				fmt.Fprintf(file, ", ")
			}
			fmt.Fprintf(file, "`@%s`", f)
		}
		fmt.Fprintf(file, "\n\n")
	}

	// Template repos
	fmt.Fprintf(file, "## 📦 Template Repositories (%d)\n\n", len(report.TemplateRepos))
	fmt.Fprintf(file, "<details>\n<summary>Click to expand</summary>\n\n")
	for _, repo := range report.TemplateRepos {
		fmt.Fprintf(file, "- `%s`\n", repo)
	}
	fmt.Fprintf(file, "\n</details>\n\n")

	// Organization results
	if report.SuccessCount > 0 {
		fmt.Fprintf(file, "## ✅ Successfully Created Organizations (%d)\n\n", report.SuccessCount)
		fmt.Fprintf(file, "<details>\n<summary>Click to expand</summary>\n\n")
		fmt.Fprintf(file, "| Organization | User | Repos Created | Repos Failed |\n")
		fmt.Fprintf(file, "|--------------|------|-------------:|--------------:|\n")

		for _, org := range report.Organizations {
			if org.Status == "success" {
				successRepos := 0
				failedRepos := 0
				for _, repo := range org.Repositories {
					if repo.Status == "success" {
						successRepos++
					} else {
						failedRepos++
					}
				}

				emoji := "✅"
				if failedRepos > 0 {
					emoji = "⚠️"
				}

				fmt.Fprintf(file, "| %s `%s` | `@%s` | %d | %d |\n",
					emoji, org.OrgName, org.User, successRepos, failedRepos)
			}
		}
		fmt.Fprintf(file, "\n</details>\n\n")
	}

	// Failed organizations
	if report.FailureCount > 0 {
		fmt.Fprintf(file, "## ❌ Failed Organizations (%d)\n\n", report.FailureCount)
		fmt.Fprintf(file, "| Organization | User | Error |\n")
		fmt.Fprintf(file, "|--------------|------|-------|\n")

		for _, org := range report.Organizations {
			if org.Status == "failed" {
				// Truncate long error messages
				errorMsg := org.Error
				if len(errorMsg) > 80 {
					errorMsg = errorMsg[:77] + "..."
				}
				fmt.Fprintf(file, "| `%s` | `@%s` | %s |\n", org.OrgName, org.User, errorMsg)
			}
		}
		fmt.Fprintf(file, "\n")
	}

	// Repository details (collapsible)
	fmt.Fprintf(file, "## 📁 Repository Details\n\n")
	fmt.Fprintf(file, "<details>\n<summary>Click to expand detailed repository status</summary>\n\n")

	for _, org := range report.Organizations {
		if org.Status == "success" && len(org.Repositories) > 0 {
			fmt.Fprintf(file, "### `%s` (@%s)\n\n", org.OrgName, org.User)

			for _, repo := range org.Repositories {
				if repo.Status == "success" {
					fmt.Fprintf(file, "- ✅ [%s](%s)\n", repo.Name, repo.URL)
				} else {
					fmt.Fprintf(file, "- ❌ `%s` - %s\n", repo.Name, repo.Error)
				}
			}
			fmt.Fprintf(file, "\n")
		}
	}

	fmt.Fprintf(file, "</details>\n\n")

	// Footer
	fmt.Fprintf(file, "---\n\n")
	fmt.Fprintf(file, "*Generated at: %s*\n", report.GeneratedAt.Format("2006-01-02 15:04:05 MST"))

	fmt.Printf("  📊 GitHub Actions Summary: Written to step summary\n")

	return nil
}

func generateMarkdownReport(report *LabReport, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create Markdown report file: %w", err)
	}
	defer file.Close()

	// Write header
	fmt.Fprintf(file, "# Lab Environment Report\n\n")
	fmt.Fprintf(file, "**Generated:** %s\n\n", report.GeneratedAt.Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(file, "**Lab Date:** %s\n\n", report.LabDate)
	fmt.Fprintf(file, "**Enterprise:** %s\n\n", report.EnterpriseSlug)

	if len(report.Facilitators) > 0 {
		fmt.Fprintf(file, "**Facilitators:** ")
		for i, f := range report.Facilitators {
			if i > 0 {
				fmt.Fprintf(file, ", ")
			}
			fmt.Fprintf(file, "@%s", f)
		}
		fmt.Fprintf(file, "\n\n")
	}

	// Write invalid users warning if any
	if len(report.InvalidUsers) > 0 || len(report.InvalidFacilitators) > 0 {
		fmt.Fprintf(file, "## ⚠️ Invalid Users Skipped\n\n")
		if len(report.InvalidUsers) > 0 {
			fmt.Fprintf(file, "**Invalid Users (%d):** ", len(report.InvalidUsers))
			for i, u := range report.InvalidUsers {
				if i > 0 {
					fmt.Fprintf(file, ", ")
				}
				fmt.Fprintf(file, "@%s", u)
			}
			fmt.Fprintf(file, "\n\n")
		}
		if len(report.InvalidFacilitators) > 0 {
			fmt.Fprintf(file, "**Invalid Facilitators (%d):** ", len(report.InvalidFacilitators))
			for i, f := range report.InvalidFacilitators {
				if i > 0 {
					fmt.Fprintf(file, ", ")
				}
				fmt.Fprintf(file, "@%s", f)
			}
			fmt.Fprintf(file, "\n\n")
		}
	}

	// Write summary
	fmt.Fprintf(file, "## Summary\n\n")
	fmt.Fprintf(file, "- **Total Users:** %d\n", report.TotalUsers)
	fmt.Fprintf(file, "- **Successful Organizations:** %d\n", report.SuccessCount)
	fmt.Fprintf(file, "- **Failed Organizations:** %d\n", report.FailureCount)
	fmt.Fprintf(file, "- **Success Rate:** %.1f%%\n\n", float64(report.SuccessCount)/float64(report.TotalUsers)*100)

	// Write template repositories
	fmt.Fprintf(file, "## Template Repositories\n\n")
	for _, repo := range report.TemplateRepos {
		fmt.Fprintf(file, "- `%s`\n", repo)
	}
	fmt.Fprintf(file, "\n")

	// Write successful organizations
	if report.SuccessCount > 0 {
		fmt.Fprintf(file, "## ✅ Successfully Created Organizations\n\n")
		for _, org := range report.Organizations {
			if org.Status == "success" {
				fmt.Fprintf(file, "### %s\n\n", org.OrgName)
				fmt.Fprintf(file, "- **User:** @%s\n", org.User)
				fmt.Fprintf(file, "- **Created At:** %s\n", org.CreatedAt.Format("2006-01-02 15:04:05 MST"))

				successRepos := 0
				failedRepos := 0
				for _, repo := range org.Repositories {
					if repo.Status == "success" {
						successRepos++
					} else {
						failedRepos++
					}
				}
				fmt.Fprintf(file, "- **Repositories:** %d created, %d failed\n\n", successRepos, failedRepos)

				if len(org.Repositories) > 0 {
					fmt.Fprintf(file, "#### Repositories:\n\n")
					for _, repo := range org.Repositories {
						if repo.Status == "success" {
							fmt.Fprintf(file, "- ✅ `%s` - [%s](%s)\n", repo.Name, repo.URL, repo.URL)
						} else {
							fmt.Fprintf(file, "- ❌ `%s` - Error: %s\n", repo.Name, repo.Error)
						}
					}
					fmt.Fprintf(file, "\n")
				}
			}
		}
	}

	// Write failed organizations
	if report.FailureCount > 0 {
		fmt.Fprintf(file, "## ❌ Failed Organizations\n\n")
		for _, org := range report.Organizations {
			if org.Status == "failed" {
				fmt.Fprintf(file, "### %s\n\n", org.OrgName)
				fmt.Fprintf(file, "- **User:** @%s\n", org.User)
				fmt.Fprintf(file, "- **Error:** %s\n\n", org.Error)
			}
		}
	}

	return nil
}

// GenerateDeleteReportFiles generates Markdown report and GitHub Actions summary for deletions
func GenerateDeleteReportFiles(report *DeleteLabReport, outputDir string) error {
	if outputDir == "" {
		outputDir = "."
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("lab-delete-report-%s-%s.md", report.LabDate, timestamp)
	mdPath := filepath.Join(outputDir, filename)

	// Generate Markdown report
	if err := generateDeleteMarkdownReport(report, mdPath); err != nil {
		return err
	}

	// Generate GitHub Actions Step Summary if running in Actions
	if err := generateDeleteGitHubStepSummary(report); err != nil {
		// Don't fail if we can't write to step summary
		fmt.Fprintf(os.Stderr, "Warning: Failed to write GitHub step summary: %v\n", err)
	}

	fmt.Printf("\n✅ Deletion report generated successfully:\n")
	fmt.Printf("  📝 Markdown: %s\n", mdPath)

	return nil
}

// generateDeleteGitHubStepSummary writes a deletion summary to GitHub Actions UI
func generateDeleteGitHubStepSummary(report *DeleteLabReport) error {
	stepSummaryPath := os.Getenv("GITHUB_STEP_SUMMARY")
	if stepSummaryPath == "" {
		// Not running in GitHub Actions, skip
		return nil
	}

	file, err := os.OpenFile(stepSummaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write beautiful markdown summary
	fmt.Fprintf(file, "# 🗑️ Lab Environment Deletion Report\n\n")

	// Summary badges/stats
	successRate := float64(report.SuccessCount) / float64(report.TotalUsers) * 100
	emoji := "✅"
	if successRate < 100 {
		emoji = "⚠️"
	}
	if successRate < 50 {
		emoji = "❌"
	}

	fmt.Fprintf(file, "> %s **Lab Date:** `%s` | **Enterprise:** `%s`\n\n", emoji, report.LabDate, report.EnterpriseSlug)

	// Stats table
	fmt.Fprintf(file, "## 📊 Summary\n\n")
	fmt.Fprintf(file, "| Metric | Count | Percentage |\n")
	fmt.Fprintf(file, "|--------|------:|-----------:|\n")
	fmt.Fprintf(file, "| **Total Organizations** | %d | 100%% |\n", report.TotalUsers)
	fmt.Fprintf(file, "| ✅ **Successfully Deleted** | %d | %.1f%% |\n", report.SuccessCount, successRate)
	fmt.Fprintf(file, "| ❌ **Failed to Delete** | %d | %.1f%% |\n", report.FailureCount,
		float64(report.FailureCount)/float64(report.TotalUsers)*100)
	fmt.Fprintf(file, "\n")

	// Facilitators
	if len(report.Facilitators) > 0 {
		fmt.Fprintf(file, "**👥 Facilitators:** ")
		for i, f := range report.Facilitators {
			if i > 0 {
				fmt.Fprintf(file, ", ")
			}
			fmt.Fprintf(file, "`@%s`", f)
		}
		fmt.Fprintf(file, "\n\n")
	}

	// Organization results
	if report.SuccessCount > 0 {
		fmt.Fprintf(file, "## ✅ Successfully Deleted Organizations (%d)\n\n", report.SuccessCount)
		fmt.Fprintf(file, "| Organization | User | Deleted At |\n")
		fmt.Fprintf(file, "|--------------|------|------------|\n")

		for _, org := range report.Organizations {
			if org.Status == "success" {
				fmt.Fprintf(file, "| ✅ `%s` | `@%s` | %s |\n",
					org.OrgName, org.User, org.DeletedAt.Format("2006-01-02 15:04:05 MST"))
			}
		}
		fmt.Fprintf(file, "\n")
	}

	// Failed organizations
	if report.FailureCount > 0 {
		fmt.Fprintf(file, "## ❌ Failed to Delete Organizations (%d)\n\n", report.FailureCount)
		fmt.Fprintf(file, "| Organization | User | Error |\n")
		fmt.Fprintf(file, "|--------------|------|-------|\n")

		for _, org := range report.Organizations {
			if org.Status == "failed" {
				// Truncate long error messages
				errorMsg := org.Error
				if len(errorMsg) > 80 {
					errorMsg = errorMsg[:77] + "..."
				}
				fmt.Fprintf(file, "| ❌ `%s` | `@%s` | %s |\n", org.OrgName, org.User, errorMsg)
			}
		}
		fmt.Fprintf(file, "\n")
	}

	// Footer
	fmt.Fprintf(file, "---\n\n")
	fmt.Fprintf(file, "*Generated at: %s*\n", report.GeneratedAt.Format("2006-01-02 15:04:05 MST"))

	fmt.Printf("  📊 GitHub Actions Summary: Written to step summary\n")

	return nil
}

func generateDeleteMarkdownReport(report *DeleteLabReport, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create Markdown report file: %w", err)
	}
	defer file.Close()

	// Write header
	fmt.Fprintf(file, "# Lab Environment Deletion Report\n\n")
	fmt.Fprintf(file, "**Generated:** %s\n\n", report.GeneratedAt.Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(file, "**Lab Date:** %s\n\n", report.LabDate)
	fmt.Fprintf(file, "**Enterprise:** %s\n\n", report.EnterpriseSlug)

	if len(report.Facilitators) > 0 {
		fmt.Fprintf(file, "**Facilitators:** ")
		for i, f := range report.Facilitators {
			if i > 0 {
				fmt.Fprintf(file, ", ")
			}
			fmt.Fprintf(file, "@%s", f)
		}
		fmt.Fprintf(file, "\n\n")
	}

	// Write summary
	fmt.Fprintf(file, "## Summary\n\n")
	fmt.Fprintf(file, "- **Total Organizations:** %d\n", report.TotalUsers)
	fmt.Fprintf(file, "- **Successfully Deleted:** %d\n", report.SuccessCount)
	fmt.Fprintf(file, "- **Failed to Delete:** %d\n", report.FailureCount)
	fmt.Fprintf(file, "- **Success Rate:** %.1f%%\n\n", float64(report.SuccessCount)/float64(report.TotalUsers)*100)

	// Write successfully deleted organizations
	if report.SuccessCount > 0 {
		fmt.Fprintf(file, "## ✅ Successfully Deleted Organizations\n\n")
		for _, org := range report.Organizations {
			if org.Status == "success" {
				fmt.Fprintf(file, "### %s\n\n", org.OrgName)
				fmt.Fprintf(file, "- **User:** @%s\n", org.User)
				fmt.Fprintf(file, "- **Deleted At:** %s\n\n", org.DeletedAt.Format("2006-01-02 15:04:05 MST"))
			}
		}
	}

	// Write failed organizations
	if report.FailureCount > 0 {
		fmt.Fprintf(file, "## ❌ Failed to Delete Organizations\n\n")
		for _, org := range report.Organizations {
			if org.Status == "failed" {
				fmt.Fprintf(file, "### %s\n\n", org.OrgName)
				fmt.Fprintf(file, "- **User:** @%s\n", org.User)
				fmt.Fprintf(file, "- **Error:** %s\n\n", org.Error)
			}
		}
	}

	return nil
}
