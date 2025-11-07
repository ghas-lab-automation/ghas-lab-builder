package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
)

type InstallationTokenInfo struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	ClientID  string `json:"client_id"`
	AppID     string `json:"app_id"`
}

// TokenService handles GitHub App authentication
type TokenService struct {
	appID      string
	privateKey string // Changed from privateKeyPath
	baseURL    string
}

// Installation represents a GitHub App installation
type Installation struct {
	ID      int64 `json:"id"` // Changed from string to int64
	Account struct {
		Login string `json:"login"`
	} `json:"account"`
	TargetType string `json:"target_type"`
	ClientID   string `json:"client_id"`
}

// InstallationToken represents the response from the installation token endpoint
type InstallationToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewTokenService creates a new TokenService
func NewTokenService(appID, privateKey, baseURL string) *TokenService {
	return &TokenService{
		appID:      appID,
		privateKey: privateKey, // Changed
		baseURL:    baseURL,
	}
}

// CreateJWT generates a JWT for GitHub App authentication
func (ts *TokenService) CreateJWT() (string, error) {
	// Use the private key content directly
	privateKeyData := []byte(ts.privateKey) // Changed

	// Parse the PEM encoded private key
	block, _ := pem.Decode(privateKeyData)
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block from private key")
	}

	// Parse the RSA private key
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try parsing as PKCS8

		type InstallationTokenInfo struct {
			Token     string `json:"token"`
			ExpiresAt string `json:"expires_at"`
			ClientID  string `json:"client_id"`
			AppID     string `json:"app_id"`
		}
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return "", fmt.Errorf("failed to parse private key: %w", err)
		}
		var ok bool
		privateKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return "", fmt.Errorf("not an RSA private key")
		}
	}

	// Create the JWT claims
	now := time.Now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)), // Issues 60 seconds in the past
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),  // Expires in 10 minutes (GitHub maximum)
		Issuer:    ts.appID,
	}

	// Create the token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	// Sign the token with the private key
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	return tokenString, nil
}

// GetInstallations retrieves all installations for the GitHub App
func (ts *TokenService) GetInstallations(jwt string) ([]Installation, error) {
	var allInstallations []Installation
	page := 1
	perPage := 100

	for {
		url := fmt.Sprintf("%s/app/installations?per_page=%d&page=%d", ts.baseURL, perPage, page)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt))
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get installations: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
		}

		var installations []Installation
		if err := json.NewDecoder(resp.Body).Decode(&installations); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode installations response: %w", err)
		}
		resp.Body.Close()

		// If no installations returned, we've reached the end
		if len(installations) == 0 {
			break
		}

		allInstallations = append(allInstallations, installations...)

		// If we got fewer results than requested, we've reached the last page
		if len(installations) < perPage {
			break
		}

		page++
	}

	return allInstallations, nil
}

// CreateInstallationToken creates an installation access token
func (ts *TokenService) CreateInstallationToken(jwt string, installationID int64) (*InstallationToken, error) {
	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", ts.baseURL, installationID)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var token InstallationToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("failed to decode installation token response: %w", err)
	}

	return &token, nil
}

// GetInstallationToken is a convenience method that creates a JWT and exchanges it for an installation token
func (ts *TokenService) GetInstallationToken(tokenType string) (InstallationTokenInfo, error) {
	// Create JWT
	jwt, err := ts.CreateJWT()
	if err != nil {
		return InstallationTokenInfo{}, fmt.Errorf("failed to create JWT: %w", err)
	}

	// Get installations
	installations, err := ts.GetInstallations(jwt)
	if err != nil {
		return InstallationTokenInfo{}, fmt.Errorf("failed to get installations: %w", err)
	}

	if len(installations) == 0 {
		return InstallationTokenInfo{}, fmt.Errorf("no installations found for this GitHub App")
	}

	// Use the installation with token_type "Enterprise"
	var installationID int64
	var clientID string

	for _, installation := range installations {
		if installation.TargetType == tokenType {
			installationID = installation.ID
			clientID = installation.ClientID
			break
		}
	}

	if installationID == 0 { // Changed from "" to 0
		return InstallationTokenInfo{}, fmt.Errorf("no suitable installation found for this GitHub App")
	}
	// Create installation token
	token, err := ts.CreateInstallationToken(jwt, installationID)
	if err != nil {
		return InstallationTokenInfo{}, fmt.Errorf("failed to create installation token: %w", err)
	}

	installationToken := InstallationTokenInfo{
		Token:     token.Token,
		ExpiresAt: token.ExpiresAt.Format(time.RFC3339),
		ClientID:  clientID,
		AppID:     fmt.Sprintf("%d", installationID), // Convert int64 to string
	}

	return installationToken, nil
}

// GetInstallationTokenForOrg gets an installation token for a specific organization
func (ts *TokenService) GetInstallationTokenForOrg(orgLogin string) (string, error) {
	// Create JWT
	jwt, err := ts.CreateJWT()
	if err != nil {
		return "", fmt.Errorf("failed to create JWT: %w", err)
	}

	// Get installations
	installations, err := ts.GetInstallations(jwt)
	if err != nil {
		return "", fmt.Errorf("failed to get installations: %w", err)
	}

	// Find the installation for the specified organization
	var installationID int64 // Changed from string to int64
	for _, installation := range installations {
		if installation.Account.Login == orgLogin {
			installationID = installation.ID
			break
		}
	}

	if installationID == 0 { // Changed from "" to 0
		return "", fmt.Errorf("no installation found for organization: %s", orgLogin)
	}

	// Create installation token
	token, err := ts.CreateInstallationToken(jwt, installationID)
	if err != nil {
		return "", fmt.Errorf("failed to create installation token: %w", err)
	}

	return token.Token, nil
}
