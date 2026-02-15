package fflogs

import (
	"context"
)

type RateLimitData struct {
	LimitPerHour        int `json:"limitPerHour"`
	PointsSpentThisHour int `json:"pointsSpentThisHour"`
	PointsResetIn       int `json:"pointsResetIn"`
}

type RateLimitResponse struct {
	RateLimitData RateLimitData `json:"rateLimitData"`
}

func (c *LogsClient) GetRateLimitData(ctx context.Context) (*RateLimitData, error) {
	query := `
		query {
			rateLimitData {
				limitPerHour
				pointsSpentThisHour
				pointsResetIn
			}
		}
	`

	var resp RateLimitResponse
	err := c.Query(ctx, query, nil, &resp)
	if err != nil {
		return nil, err
	}

	return &resp.RateLimitData, nil
}
