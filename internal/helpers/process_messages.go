package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/gocql/gocql"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/pb"
)

type ProductOutbox struct {
	Id        gocql.UUID
	Bucket    string
	EventType string
	Payload   string
}

var (
	getProductsFromOutboxQuery = `
		SELECT id, bucket, payload, event_type 
		FROM products_keyspace_v2.products_outbox 
		WHERE bucket = ? 
		ORDER BY id ASC;
	`
)

func ProcessMessages(ctx context.Context, session *gocql.Session, producer pulsar.Producer) error {
	bucket := getCurrentBucket()

	messages, err := fetchMessages(ctx, session, bucket)
	if err != nil {
		return fmt.Errorf("failed to fetch messages: %w", err)
	}

	for _, message := range messages {
		if err := sendToPulsar(ctx, producer, message, session); err != nil {
			slog.Error("Failed to send message to Pulsar", "error", err, "messageID", message.Id)
			continue
		}
	}
	return nil
}

func fetchMessages(ctx context.Context, session *gocql.Session, bucket string) ([]ProductOutbox, error) {
	var productsData []ProductOutbox

	iter := session.Query(getProductsFromOutboxQuery, bucket).WithContext(ctx).Iter()
	defer iter.Close()

	for {
		var newProduct ProductOutbox
		if !iter.Scan(&newProduct.Id, &newProduct.Bucket, &newProduct.Payload, &newProduct.EventType) {
			break
		}
		productsData = append(productsData, newProduct)
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}

	return productsData, nil
}

func sendToPulsar(ctx context.Context, producer pulsar.Producer, message ProductOutbox, session *gocql.Session) error {
	var product pb.Product
	if err := json.Unmarshal([]byte(message.Payload), &product); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	payload, err := json.Marshal(&product)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	messageChan := make(chan error, 1)

	producer.SendAsync(ctx, &pulsar.ProducerMessage{
		Key:     fmt.Sprintf("%s:%d", message.EventType, product.Id),
		Payload: payload,
	}, func(_ pulsar.MessageID, _ *pulsar.ProducerMessage, err error) {
		select {
		case messageChan <- err:
		default:
		}
		close(messageChan)
	})

	select {
	case err, ok := <-messageChan:
		if !ok {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to publish message: %w", err)
		}
	case <-ctx.Done():
		return fmt.Errorf("context canceled while publishing message")
	}

	slog.Info("Message sent to Pulsar", "messageID", message.Id)

	return deleteMessage(ctx, session, message.Id)
}

func deleteMessage(ctx context.Context, session *gocql.Session, msgID gocql.UUID) error {
	bucket := getCurrentBucket()

	slog.Info("Deleting message", "messageID", msgID)

	return session.Query(`
		DELETE FROM products_keyspace_v2.products_outbox WHERE bucket = ? AND id = ?`,
		bucket, msgID,
	).WithContext(ctx).Exec()
}

func getCurrentBucket() string {
	return time.Now().Format("2006-01-02")
}
