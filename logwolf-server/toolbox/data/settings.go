package data

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const defaultRetentionDays = 90

// ValidRetentionDays are the only values accepted by the UI and API.
var ValidRetentionDays = map[int]bool{
	30: true, 60: true, 90: true, 180: true, 365: true, 0: true, // 0 = forever
}

type Settings struct {
	client *mongo.Client
}

type settingsDoc struct {
	ProjectID string `bson:"project_id"`
	Key       string `bson:"key"`
	Value     int    `bson:"value"`
}

// RetentionArgs is the RPC argument for GetRetention and UpdateRetention.
type RetentionArgs struct {
	ProjectID string
	Days      int
}

func (s *Settings) collection() *mongo.Collection {
	return s.client.Database("logs").Collection("settings")
}

func (s *Settings) GetRetentionDays(projectID string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var doc settingsDoc
	err := s.collection().FindOne(ctx, bson.M{"project_id": projectID, "key": "retention_days"}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return defaultRetentionDays, nil
	}
	if err != nil {
		return 0, fmt.Errorf("GetRetentionDays: %w", err)
	}
	return doc.Value, nil
}

func (s *Settings) SetRetentionDays(projectID string, days int) error {
	if !ValidRetentionDays[days] {
		return fmt.Errorf("SetRetentionDays: %d is not a valid retention value", days)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.collection().UpdateOne(
		ctx,
		bson.M{"project_id": projectID, "key": "retention_days"},
		bson.M{"$set": bson.M{"project_id": projectID, "key": "retention_days", "value": days}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("SetRetentionDays: %w", err)
	}
	return nil
}

// EnsureSettingsIndex creates a compound unique index on (project_id, key).
func (s *Settings) EnsureSettingsIndex() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := s.collection().Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "project_id", Value: 1}, {Key: "key", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("unique_project_key"),
	})
	if err != nil {
		return fmt.Errorf("EnsureSettingsIndex: %w", err)
	}
	return nil
}

// EnsureTTLIndex creates or updates the TTL index on the logs collection.
// days=0 means retain forever — the index is dropped if it exists.
func (s *Settings) EnsureTTLIndex(days int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	coll := s.client.Database("logs").Collection("logs")
	const indexName = "ttl_created_at"

	// Check if the index already exists
	cursor, err := coll.Indexes().List(ctx)
	if err != nil {
		return fmt.Errorf("EnsureTTLIndex list: %w", err)
	}
	defer cursor.Close(ctx)

	indexExists := false
	for cursor.Next(ctx) {
		var idx bson.M
		if err := cursor.Decode(&idx); err != nil {
			continue
		}
		if idx["name"] == indexName {
			indexExists = true
			break
		}
	}

	// days=0: retain forever — drop existing TTL index if present
	if days == 0 {
		if indexExists {
			if _, err := coll.Indexes().DropOne(ctx, indexName); err != nil {
				return fmt.Errorf("EnsureTTLIndex drop: %w", err)
			}
			log.Println("TTL index removed: logs will be retained forever")
		}
		return nil
	}

	expireAfterSeconds := int32(days * 24 * 60 * 60)

	if indexExists {
		// Update existing index via collMod
		db := s.client.Database("logs")
		cmd := bson.D{
			{Key: "collMod", Value: "logs"},
			{Key: "index", Value: bson.D{
				{Key: "name", Value: indexName},
				{Key: "expireAfterSeconds", Value: expireAfterSeconds},
			}},
		}
		if err := db.RunCommand(ctx, cmd).Err(); err != nil {
			return fmt.Errorf("EnsureTTLIndex collMod: %w", err)
		}
		log.Printf("TTL index updated: logs older than %d days will be purged", days)
		return nil
	}

	// Create fresh index
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(expireAfterSeconds).SetName(indexName),
	}
	if _, err := coll.Indexes().CreateOne(ctx, indexModel); err != nil {
		return fmt.Errorf("EnsureTTLIndex create: %w", err)
	}
	log.Printf("TTL index created: logs older than %d days will be purged", days)
	return nil
}
