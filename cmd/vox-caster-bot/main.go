package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os/signal"
	"syscall"

	"vox-caster-bot/internal/bot"
	"vox-caster-bot/internal/config"
	"vox-caster-bot/internal/feed"
	"vox-caster-bot/internal/state"
	"vox-caster-bot/internal/telegram"
	"vox-caster-bot/internal/wiki"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	once := flag.Bool("once", false, "poll once and exit instead of running the loop")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	httpClient, err := buildHTTPClient(cfg.InsecureSkipVerify, cfg.ProxyURL)
	if err != nil {
		log.Fatalf("build http client: %v", err)
	}

	store, err := state.NewFileStore(cfg.StatePath, cfg.StateMaxAge)
	if err != nil {
		log.Fatalf("load state: %v", err)
	}

	b := &bot.Bot{
		Feeds:     cfg.Feeds,
		ChannelID: cfg.ChannelID,
		Interval:  cfg.PollInterval,
		Fetcher:   feed.NewHTTPFetcher(httpClient),
		State:     store,
		Telegram:  telegram.NewClient(cfg.TelegramToken, httpClient),
	}

	if cfg.WikiAPI != "" {
		b.Wiki = wiki.NewClient(cfg.WikiAPI, httpClient)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if *once {
		b.Poll(ctx)
		return
	}

	if err := b.Run(ctx); err != nil {
		log.Fatalf("bot error: %v", err)
	}
}

func buildHTTPClient(insecureSkipVerify bool, proxyURL string) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if proxyURL != "" {
		u, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("parse proxy_url: %w", err)
		}
		transport.Proxy = http.ProxyURL(u)
	}

	if insecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return &http.Client{Transport: transport}, nil
}
