package helpers

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/gocql/gocql"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ProductEvent struct {
	Id        int64
	Bucket    string
	EventType string
	Payload   string
}

type ProductPayload struct {
	ID          int64
	Name        string
	Description string
	Price       float64
	CategoryId  int64
	Tags        []string
	CreatedAt   *timestamppb.Timestamp
	UpdatedAt   *timestamppb.Timestamp
}

func ConsumeMessages(ctx context.Context, consumer pulsar.Consumer, session *gocql.Session) {
	for {
		select {
		case <-ctx.Done():
			slog.Info("Shutting down message consumer")
			return
		default:

			msg, err := consumer.Receive(ctx)
			if err != nil {
				slog.Error("Failed to receive message", "error", err)
				continue
			}

			var product pb.Product
			if err := json.Unmarshal(msg.Payload(), &product); err != nil {
				slog.With("message_id", msg.ID()).Error("Failed to unmarshal product", "error", err)
				consumer.Nack(msg) // Added to avoid processing broken messages
				continue
			}

			goTime := product.CreatedAt.AsTime()

			slog.With("product_id", product.Id).Info("Received inventory message")

			if err := SaveInventory(ctx, session, product.Id, product.CategoryId, product.Stock, goTime); err != nil {
				slog.With("product_id", product.Id).Error("Failed to save inventory", "error", err)
				consumer.Nack(msg)
				continue
			}

			consumer.Ack(msg)
		}
	}
}

func SaveInventory(ctx context.Context, session *gocql.Session, productId int64, categoryId int64, stock int32, createdAt time.Time) error {
	query := `INSERT INTO products_keyspace_v2.inventory 
              (product_id, category_id, stock_count, created_at, last_updated_at) 
              VALUES (?, ?, ?, ?, ?)`

	return session.Query(query, productId, categoryId, stock, createdAt, time.Now()).
		WithContext(ctx).
		Exec()
}
