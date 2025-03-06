package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/internal/database"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/internal/helpers"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/internal/pkg"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/internal/queue"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/internal/snowflake"
)

// grpc server

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var cfg pkg.Config
	file, err := os.Open("config.yaml")
	if err != nil {
		slog.Error("failed to open config.yaml", "error", err)
		os.Exit(1)
	}

	defer file.Close()

	if err := cfg.LoadConfig(file); err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)

	}

	if err := snowflake.InitSonyFlake(); err != nil {
		slog.Error("failed to initialize snowflake", "error", err)
		os.Exit(1)
	}
	astraCfg := &database.AstraConfig{
		Username: cfg.Database.Username,
		Path:     cfg.Database.Path,
		Token:    helpers.GetEnvOrDefault("DATABASE_TOKEN", ""),
	}

	db := database.NewAstraDB()
	session, err := db.Connect(ctx, astraCfg, 30)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer session.Close()
	pulsarCfg := &queue.PulsarConfig{
		URI:       cfg.Queue.URI,
		TopicName: cfg.Queue.Topic,
		Token:     helpers.GetEnvOrDefault("PULSAR_TOKEN", ""),
	}

	queueInstance := queue.NewPulsar(pulsarCfg)
	client, err := queueInstance.CreatePulsarConnection(ctx)
	if err != nil {
		slog.Error("failed to create pulsar connection", "error", err)
		os.Exit(1)
	}
	defer client.Close()

	producer, err := queueInstance.CreatePulsarProducer(ctx, client)
	if err != nil {
		slog.Error("failed to create pulsar producer", "error", err)
		os.Exit(1)
	}
	defer producer.Close()

}
