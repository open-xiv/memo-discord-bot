package fflogs

import (
	"context"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2/clientcredentials"
)

type LogsClient struct {
	graphql *graphql.Client
}

func NewLogsClient(client, secret string) *LogsClient {
	if client == "" || secret == "" {
		log.Fatal().Msgf("client ID or secret not set")
	}

	config := &clientcredentials.Config{
		ClientID:     client,
		ClientSecret: secret,
		TokenURL:     "https://www.fflogs.com/oauth/token",
	}

	oauthClient := config.Client(context.Background())

	return &LogsClient{
		graphql: graphql.NewClient(
			"https://www.fflogs.com/api/v2/client",
			graphql.WithHTTPClient(oauthClient),
		),
	}
}

func (c *LogsClient) Query(ctx context.Context, query string, vars map[string]any, res any) error {
	req := graphql.NewRequest(query)
	for k, v := range vars {
		req.Var(k, v)
	}

	err := c.graphql.Run(ctx, req, res)
	if err != nil {
		log.Error().Err(err).Str("query", query).Interface("vars", vars).Msg("GraphQL query failed")
	}
	return err
}
