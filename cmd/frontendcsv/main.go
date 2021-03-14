package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/torfjor/kindly/cmd/frontendcsv/server"
	"github.com/torfjor/kindly/statistics"
	"github.com/torfjor/kindly/statistics/auth"
	"golang.org/x/oauth2"
)

type config struct {
	listenPort string
	botID      string
	apiKey     string
}

func main() {
	listenPortFlag := flag.String("port", "8080", "HTTP listen port")
	botIDFlag := flag.String("botid", "", "kindly bot ID")
	apiKeyFlag := flag.String("apikey", "", "kindly API key")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := run(ctx, &config{
		listenPort: *listenPortFlag,
		botID:      *botIDFlag,
		apiKey:     *apiKeyFlag,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context, config *config) error {
	client := &statistics.Client{
		BotID: config.botID,
		Doer: oauth2.NewClient(context.Background(), oauth2.ReuseTokenSource(nil, &auth.TokenSource{
			APIKey: config.apiKey,
			BotID:  config.botID,
		})),
	}

	srv := http.Server{
		Addr:         ":" + config.listenPort,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  30 * time.Second,
		Handler:      server.NewServer(client),
	}

	go func() {
		select {
		case <-ctx.Done():
			srv.Shutdown(context.Background())
		}
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	return nil
}
