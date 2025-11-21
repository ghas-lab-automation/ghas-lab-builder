# GHAS Lab Builder

A CLI tool for automating the creation and management of GitHub Advanced Security (GHAS) lab environments. This tool helps instructors and facilitators quickly provision multiple organizations with pre-configured repositories for hands-on training sessions.

## Features

- **Automated Lab Environment Provisioning**: Create complete lab environments with organizations and repositories for multiple users
- **Parallel Processing**: Efficiently provision resources using concurrent workers (up to 9 parallel operations)
- **GitHub App Integration**: Install GitHub Apps automatically on newly created organizations
- **Template Repository Support**: Clone from template repositories with optional branch inclusion
- **User Validation**: Validate GitHub usernames before provisioning
- **Facilitator Support**: Separate handling for lab facilitators with their own organizations
- **Granular Control**: Manage individual organizations and repositories independently
- **Comprehensive Reporting**: Generate detailed reports of lab creation and deletion operations
- **Flexible Authentication**: Support for both Personal Access Tokens (PAT) and GitHub App authentication
- **Enterprise Support**: Works with GitHub Enterprise Cloud and Server

## Prerequisites

- Go 1.25.2 or higher
- GitHub Enterprise account
- One of the following authentication methods:
  - Personal Access Token (PAT) with appropriate permissions
  - GitHub App credentials (App ID and Private Key)

## Installation

```bash
# Clone the repository
git clone https://github.com/ghas-lab-automation/ghas-lab-builder.git
cd ghas-lab-builder

# Build the binary
go build -o ghas-lab-builder

# Or install directly
go install
```

## Authentication

The tool supports two authentication methods:

### Personal Access Token (PAT)
```bash
--token YOUR_GITHUB_TOKEN
```
Note: The GitHub rate limit restricts a user to 50 repos per minute and 150 repos per hour.

### GitHub App
```bash
--app-id YOUR_APP_ID --private-key /path/to/private-key.pem
```

**Note**: You must use either `--token` OR both `--app-id` and `--private-key`, but not both simultaneously.

## Usage

### Lab Commands

Lab commands provide end-to-end management of complete lab environments, including organizations and repositories for all users.

#### Create a Lab Environment

Create a complete lab environment with organizations and repositories for all users:

```bash
ghas-lab-builder lab create \
  --enterprise-slug YOUR_ENTERPRISE \
  --token YOUR_TOKEN \
  --lab-date 2025-11-07 \
  --users-file users.txt \
  --facilitators admin1,admin2 \
  --template-repos default/repos.json
```

**What this does:**
- Validates all student and facilitator usernames
- Creates organizations for each user (format: `ghas-labs-2025-11-07-username`)
- Installs the GitHub App on each organization
- Creates all template repositories in each organization
- Generates a comprehensive report

#### Delete a Lab Environment

Remove all organizations and resources created for a lab:

```bash
ghas-lab-builder lab delete \
  --enterprise-slug YOUR_ENTERPRISE \
  --token YOUR_TOKEN \
  --lab-date 2025-11-07 \
  --users-file users.txt \
  --facilitators admin1,admin2
```

**What this does:**
- Deletes all organizations created for the specified lab date
- Processes deletions in parallel for efficiency
- Generates a deletion report with success/failure details

### Organization Commands

Organization commands allow you to manage individual organizations independently.

#### Create a Single Organization

Create an organization for a specific user:

```bash
ghas-lab-builder orgs create \
  --enterprise-slug YOUR_ENTERPRISE \
  --token YOUR_TOKEN \
  --lab-date 2025-11-07 \
  --user student1 \
  --facilitators admin1,admin2
```

**What this does:**
- Validates the user and facilitators
- Creates organization named `ghas-labs-2025-11-07-student1`
- Installs the GitHub App on the organization
- Adds facilitators as organization owners
- Does NOT create any repositories (use `repo create` for that)

#### Delete a Single Organization

Delete a specific organization:

```bash
ghas-lab-builder orgs delete \
  --enterprise-slug YOUR_ENTERPRISE \
  --token YOUR_TOKEN \
  --lab-date 2025-11-07 \
  --user student1
```

**What this does:**
- Deletes the organization `ghas-labs-2025-11-07-student1`
- Removes all repositories and resources within the organization

### Repository Commands

Repository commands allow you to manage repositories within an existing organization.

#### Create Repositories in an Organization

Create repositories from templates in a specific organization:

```bash
ghas-lab-builder repo create \
  --enterprise-slug YOUR_ENTERPRISE \
  --token YOUR_TOKEN \
  --org ghas-labs-2025-11-07-student1 \
  --repos default/repos.json
```

**What this does:**
- Creates all repositories defined in the JSON file
- Clones from template repositories
- Optionally includes all branches based on configuration

#### Delete Repositories from an Organization

Delete specific repositories from an organization:

```bash
# Delete specific repositories
ghas-lab-builder repo delete \
  --enterprise-slug YOUR_ENTERPRISE \
  --token YOUR_TOKEN \
  --org ghas-labs-2025-11-07-student1 \
  --repos default/repos.json

# Delete ALL repositories in the organization
ghas-lab-builder repo delete \
  --enterprise-slug YOUR_ENTERPRISE \
  --token YOUR_TOKEN \
  --org ghas-labs-2025-11-07-student1
```

**What this does:**
- If `--repos` is specified: Deletes only the repositories listed in the JSON file
- If `--repos` is omitted: Deletes ALL repositories in the organization

### Command Options

#### Global Flags
- `--enterprise-slug`: GitHub Enterprise slug (required)
- `--token`: Personal Access Token for authentication
- `--app-id`: GitHub App ID (for App authentication)
- `--private-key`: Path to GitHub App private key file (for App authentication)
- `--base-url`: GitHub API base URL (defaults to `https://api.github.com`)

#### Lab Command Flags
- `--lab-date`: Date identifier for the lab (e.g., '2025-11-07') (required)
- `--users-file`: Path to text file containing student usernames (required)
- `--facilitators`: Comma-separated list of facilitator usernames (required)
- `--template-repos`: Path to JSON file defining template repositories (required for create)

#### Organization Command Flags
- `--lab-date`: Date identifier for the lab (e.g., '2025-11-07') (required)
- `--user`: Username for the organization (required)
- `--facilitators`: Comma-separated list of facilitator usernames (required for create)

#### Repository Command Flags
- `--org`: Organization name (required)
- `--repos`: Path to JSON file defining repositories (required for create, optional for delete)

## File Formats

### Users File (`users.txt`)

Plain text file with comma-separated GitHub usernames:

```
student1,student2,student3,student4
```

### Template Repositories File (`repos.json`)

JSON file defining template repositories to clone:

```json
{
  "lab-env-setup": {
    "repos": [
      {
        "template": "org-name/repo-name",
        "include_all_branches": false
      },
      {
        "template": "org-name/another-repo",
        "include_all_branches": true
      }
    ]
  }
}
```

**Fields:**
- `template`: Full repository path in format `owner/repo-name`
- `include_all_branches`: Whether to clone all branches (true) or only the default branch (false)

## Use Cases

### Complete Lab Setup
Use `lab create` when you need to set up a full lab environment for multiple students from scratch.

### Individual Student Setup
Use `orgs create` + `repo create` when you need to provision resources for a single student who joined late or needs a replacement environment.

### Partial Repository Reset
Use `repo delete` + `repo create` when you need to reset specific repositories without affecting the entire organization.

### Organization Management
Use `orgs delete` when you need to remove a specific student's organization without affecting others in the same lab.

## How It Works

### Lab Creation Process

1. **User Validation**: Validates all student and facilitator GitHub usernames
2. **Organization Creation**: Creates organizations named `ghas-labs-{lab-date}-{username}`
3. **GitHub App Installation**: Installs the configured GitHub App on each organization
4. **Repository Provisioning**: Creates repositories from templates in each organization
5. **Report Generation**: Creates detailed markdown and JSON reports in the `reports/` directory

### Lab Deletion Process

1. **Organization Deletion**: Removes all organizations created for the specified lab date
2. **Parallel Processing**: Uses multiple workers for efficient deletion
3. **Report Generation**: Creates deletion reports with success/failure details

### Organization Naming Convention

Organizations are created with the following naming pattern:
```
ghas-labs-{lab-date}-{username}
```

Example: `ghas-labs-2025-11-07-student1`

## Reports

The tool generates detailed reports in the `reports/` directory:

- **Lab Creation Report**: `lab-report-{lab-date}-{timestamp}.md`
- **Lab Deletion Report**: `lab-delete-report-{lab-date}-{timestamp}.md`

Reports include:
- Total user count
- Success/failure counts
- Individual organization details
- Repository creation status
- Error messages for failures
- Invalid usernames

## Logging

Logs are automatically generated and stored with timestamps:
- Format: `ghas-lab-builder-{timestamp}.log`
- Level: Info (includes errors and warnings)
- Output: Both file and console

## Project Structure

```
.
├── cmd/                      # CLI commands
│   ├── ghas_lab_builder.go  # Root command
│   ├── lab/                 # Lab environment commands
│   │   ├── create.go        # Create complete lab
│   │   ├── delete.go        # Delete complete lab
│   │   └── lab.go           # Lab command root
│   ├── orgs/                # Organization commands
│   │   ├── create.go        # Create single org
│   │   ├── delete.go        # Delete single org
│   │   └── orgs.go          # Orgs command root
│   └── repo/                # Repository commands
│       ├── create.go        # Create repos in org
│       ├── delete.go        # Delete repos from org
│       └── repo.go          # Repo command root
├── internal/
│   ├── auth/                # Authentication services
│   ├── config/              # Configuration constants
│   ├── github/              # GitHub API clients
│   ├── services/            # Business logic
│   └── util/                # Utility functions
├── default/                 # Default configuration files
├── reports/                 # Generated reports
└── scripts/                 # Helper scripts
```

## Performance

- **Concurrent Workers**: Up to 9 parallel workers for provisioning/deletion
- **Efficient Processing**: Automatically scales workers based on user count
- **User Validation**: Pre-validates all usernames to avoid failures during provisioning

## Error Handling

- Invalid usernames are reported but don't stop the provisioning process
- Failed organization/repository creations are logged and reported
- Detailed error messages in reports and logs
- Graceful handling of API rate limits and timeouts

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE.md](LICENSE.md) file for details.


## Support

For issues and questions, please open an issue on the GitHub repository.

## Authors

- s-samadi

## Acknowledgments

Built with:
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [golang-jwt](https://github.com/golang-jwt/jwt) - JWT implementation for GitHub App authentication
