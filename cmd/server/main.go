package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/internal/controller"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/internal/database"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/internal/helpers"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/internal/pkg"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/internal/queue"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/internal/snowflake"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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

	err = godotenv.Load()
	if err != nil {
		slog.Error("failed to load .env file", "error", err)
		os.Exit(1)

	}
	astraCfg := &database.AstraConfig{
		Username: cfg.Database.Username,
		Path:     cfg.Database.Path,
		Token:    helpers.GetEnvOrDefault("DATABASE_TOKEN", ""),
	}

	db := database.NewAstraDB()
	session, err := db.Connect(ctx, astraCfg, 30*time.Second)
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

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GrpcServer.Port))
	if err != nil {
		slog.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	productContoller := controller.NewProductController(session)

	server := grpc.NewServer()
	reflection.Register(server) //use server reflection, not required
	pb.RegisterProductServiceServer(server, productContoller)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	stopCH := make(chan os.Signal, 1)

	// polling approach

	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// poll messages
				if err := helpers.ProcessMessages(context.Background(), session, producer); err != nil {
					slog.Error("failed to process messages", "error", err)
					os.Exit(1)
				}
			case <-stopCH:
				return
			}

		}
	}()
	go func() {
		sig := <-sigChan
		slog.Info("Received shutdown signal", "signal", sig)
		slog.Info("Shutting down gRPC server...")

		// Gracefully stop the gRPC server
		server.GracefulStop()
		cancel()      // Cancel context for other goroutines
		close(stopCH) // Notify the polling goroutine to stop

		slog.Info("gRPC server has been stopped gracefully")
	}()

	slog.Info("Starting gRPC server", "port", cfg.GrpcServer.Port)
	if err := server.Serve(lis); err != nil {
		slog.Error("gRPC server encountered an error while serving", "error", err)
		os.Exit(1)
	}

}
