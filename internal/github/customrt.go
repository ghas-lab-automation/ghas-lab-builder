package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/s-samadi/ghas-lab-builder/internal/auth"
	"github.com/s-samadi/ghas-lab-builder/internal/config"
)

// AuthProvider fetches an Authorization header value (e.g. "Bearer <token>") for a request.
// It may consult context, request, refresh tokens, etc. If it returns "", no Authorization header is set.
// If it returns an error the RoundTrip will return that error.
type AuthProvider func(req *http.Request) (authHeaderValue string, err error)

// Options controls the behavior of the CustomRoundTripper.
type Options struct {
	// Underlying transport to call. If nil, http.DefaultTransport is used.
	Base http.RoundTripper

	// Static headers to add to every request (GitHub-style headers or others).
	// Values will be set on req.Header (overwrites any existing header with same name).
	StaticHeaders map[string]string

	// Optional function called to provide Authorization header per-request.
	AuthProvider AuthProvider

	// Logger used for structured logging. If nil, slog.Default() is used.
	Logger *slog.Logger

	// Maximum number of bytes to log for request and response bodies.
	// Set to 0 to disable body logging.
	MaxBodyLogBytes int64
}

// tokenCache holds cached tokens by target type
type tokenCache struct {
	sync.RWMutex
	tokens map[string]cachedToken
}

type cachedToken struct {
	token   string
	expires time.Time
}

var globalTokenCache = &tokenCache{
	tokens: make(map[string]cachedToken),
}

// CustomRoundTripper implements http.RoundTripper
type CustomRoundTripper struct {
	base            http.RoundTripper
	staticHeaders   map[string]string
	authProvider    AuthProvider
	logger          *slog.Logger
	maxBodyLogBytes int64
}

// NewCustomRoundTripper constructs a CustomRoundTripper with sane defaults.
func NewCustomRoundTripper(opts Options) *CustomRoundTripper {
	base := opts.Base
	if base == nil {
		base = http.DefaultTransport
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// copy static headers to avoid mutation later
	static := map[string]string{}
	for k, v := range opts.StaticHeaders {
		static[k] = v
	}

	return &CustomRoundTripper{
		base:            base,
		staticHeaders:   static,
		authProvider:    opts.AuthProvider,
		logger:          logger,
		maxBodyLogBytes: opts.MaxBodyLogBytes,
	}
}

// RoundTrip implements the http.RoundTripper interface.
func (c *CustomRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	// Create a shallow clone of request to avoid mutating caller's request headers/body
	req2 := req.Clone(req.Context())

	// Inject static headers (e.g., GitHub headers)
	for k, v := range c.staticHeaders {
		req2.Header.Set(k, v)
	}

	// Inject auth header if provider present
	if c.authProvider != nil {
		val, err := c.authProvider(req2)
		if err != nil {
			c.logger.Error("auth provider error", slog.String("method", req2.Method), slog.String("url", req2.URL.String()), slog.Any("error", err))
			return nil, err
		}
		if val != "" {
			req2.Header.Set("Authorization", val)
		}
	}

	c.logger.Info("HTTP Request",
		slog.String("method", req2.Method),
		slog.String("url", req2.URL.String()),
	)

	// Perform the actual request
	resp, err := c.base.RoundTrip(req2)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("HTTP Error",
			slog.String("method", req2.Method),
			slog.String("url", req2.URL.String()),
			slog.Any("error", err),
			slog.Duration("took", duration),
		)
		return nil, err
	}

	c.logger.Info("HTTP Response",
		slog.Int("status", resp.StatusCode),
		slog.String("method", req2.Method),
		slog.String("url", req2.URL.String()),
		slog.Duration("took", duration),
	)

	return resp, nil
}

// Helper for simple API: create a transport that injects GitHub headers and acquires token automatically
// Accepts a context with app credentials or PAT token, logger, and installation target type.
func NewGithubStyleTransport(ctx context.Context, logger *slog.Logger, targetType string) *CustomRoundTripper {
	static := map[string]string{
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
	}

	authProv := func(req *http.Request) (string, error) {
		// Check if using PAT token
		if token, ok := ctx.Value(config.TokenKey).(string); ok && token != "" {
			// Using Personal Access Token - no need for caching or installation token logic
			return "Bearer " + token, nil
		}

		// Using GitHub App authentication
		// Build cache key based on target type and organization
		cacheKey := targetType
		if targetType == config.OrganizationType {
			if orgName, ok := ctx.Value(config.OrgKey).(string); ok && orgName != "" {
				cacheKey = targetType + ":" + orgName
			}
		}

		// Check if we have a cached token for this cache key
		globalTokenCache.RLock()
		if cached, ok := globalTokenCache.tokens[cacheKey]; ok && time.Now().Before(cached.expires) {
			token := cached.token
			globalTokenCache.RUnlock()
			return "Bearer " + token, nil
		}
		globalTokenCache.RUnlock()

		// Need to get a new token
		globalTokenCache.Lock()
		defer globalTokenCache.Unlock()

		// Double-check after acquiring write lock to deal with race condition
		if cached, ok := globalTokenCache.tokens[cacheKey]; ok && time.Now().Before(cached.expires) {
			return "Bearer " + cached.token, nil
		}

		// Acquire new installation token for this target type
		ts := auth.NewTokenService(
			ctx.Value(config.AppIDKey).(string),
			ctx.Value(config.PrivateKeyKey).(string), // Changed from PrivateKeyPathKey
			ctx.Value(config.BaseURLKey).(string),
		)

		var tokenStr string
		var err error

		// For organization-specific requests, get org-specific token
		if targetType == config.OrganizationType {
			if orgName, ok := ctx.Value(config.OrgKey).(string); ok && orgName != "" {
				tokenStr, err = ts.GetInstallationTokenForOrg(orgName)
				if err != nil {
					return "", err
				}
			} else {
				// Fall back to general installation token
				token, err := ts.GetInstallationToken(targetType)
				if err != nil {
					return "", err
				}
				tokenStr = token.Token
			}
		} else {
			token, err := ts.GetInstallationToken(targetType)
			if err != nil {
				return "", err
			}
			tokenStr = token.Token
		}

		// Cache the token for 55 minutes
		globalTokenCache.tokens[cacheKey] = cachedToken{
			token:   tokenStr,
			expires: time.Now().Add(55 * time.Minute),
		}

		return "Bearer " + tokenStr, nil
	}

	return NewCustomRoundTripper(Options{
		Base:          http.DefaultTransport,
		StaticHeaders: static,
		AuthProvider:  authProv,
		Logger:        logger,
	})
}
