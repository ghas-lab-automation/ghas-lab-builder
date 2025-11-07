package services

import (
	"context"
	"log/slog"
)

func CreateOrg(ctx context.Context, logger *slog.Logger, user string) error {
	// orgName := "ghas-labs-" + user
	// logger.Info("Creating organization", slog.String("org", orgName), slog.String("user", user))

	// // Get enterprise slug from context
	// enterpriseSlug, ok := ctx.Value(config.EnterpriseSlugKey).(string)
	// if !ok {
	// 	logger.Error("Enterprise slug not found in context")
	// 	return nil
	// }

	// return api.CreateOrg(ctx, logger, user, enterpriseSlug)

	return nil
}
