package htmlstats

import (
	"context"
	"os"

	"golang.org/x/oauth2"

	"github.com/atb-as/kindly/statistics"
	"github.com/atb-as/kindly/statistics/auth"
)

func init() {
	apiKey := os.Getenv("KINDLY_API_KEY")
	botID := os.Getenv("BOT_ID")

	statsClient = statistics.NewClient(statistics.WithDoer(oauth2.NewClient(context.Background(), oauth2.ReuseTokenSource(nil, &auth.TokenSource{
		APIKey: apiKey,
		BotID:  botID,
	}))))
	statsClient.BotID = botID
}
