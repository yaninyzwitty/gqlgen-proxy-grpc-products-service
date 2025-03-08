package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/graph"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/internal/database"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/internal/helpers"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/internal/pkg"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/internal/queue"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	var cfg pkg.Config
	file, err := os.Open("config.yaml")
	if err != nil {
		slog.Error("failed to open config.yaml", "error", err)
		os.Exit(1)
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := cfg.LoadConfig(file); err != nil {
		slog.Error("failed to load config.yaml", "error", err)
		os.Exit(1)
	}

	address := fmt.Sprintf(":%d", cfg.GrpcServer.Port)
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed to create grpc client", "error", err)
		os.Exit(1)
	}

	defer conn.Close()

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

	pulsarClient, err := queueInstance.CreatePulsarConnection(ctx)
	if err != nil {
		slog.Error("failed to create pulsar connection", "error", err)
		os.Exit(1)
	}
	defer pulsarClient.Close()
	consumer, err := queueInstance.CreatePulsarConsumer(ctx, pulsarClient, cfg.Queue.Topic)
	if err != nil {
		slog.Error("failed to create pulsar consumer", "error", err)
		os.Exit(1)

	}

	defer consumer.Close()

	client := pb.NewProductServiceClient(conn)

	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: &graph.Resolver{Conn: client}}))

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	mux := chi.NewRouter()
	mux.Use(middleware.Logger)

	mux.Handle("/", playground.Handler("GraphQL playground", "/query"))
	mux.Handle("/query", srv)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.GraphqlServer.Port),
		Handler: mux,
	}

	stopCH := make(chan os.Signal, 1)
	signal.Notify(stopCH, os.Interrupt, syscall.SIGTERM)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	go func() {
		slog.Info("SERVER starting", "port", cfg.GraphqlServer.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	go helpers.ConsumeMessages(context.Background(), consumer, session)

	<-stopCH
	slog.Info("shutting down the server...")
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("failed to shutdown server", "error", err)
		os.Exit(1)
	} else {
		slog.Info("server stopped gracefully")
	}

}
