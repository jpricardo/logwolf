package data

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Models struct {
	client        *mongo.Client
	LogEntry      LogEntry
	APIKey        APIKey
	Settings      Settings
	Project       Project
	ProjectMember ProjectMember
}

type LogEntry struct {
	ID        string    `bson:"_id,omitempty" json:"id,omitempty"`
	ProjectID string    `bson:"project_id" json:"project_id"`
	Name      string    `bson:"name" json:"name"`
	Data      string    `bson:"data" json:"data"`
	Severity  string    `bson:"severity" json:"severity"`
	Tags      []string  `bson:"tags" json:"tags"`
	Duration  int       `bson:"duration,omitempty" json:"duration,omitempty"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

type LogEntryFilter struct {
	ID        string   `bson:"_id,omitempty" json:"id,omitempty"`
	ProjectID string   `bson:"project_id,omitempty" json:"project_id,omitempty"`
	Name      string   `bson:"name,omitempty" json:"name,omitempty"`
	Data      string   `bson:"data,omitempty" json:"data,omitempty"`
	Severity  string   `bson:"severity,omitempty" json:"severity,omitempty"`
	Tags      []string `bson:"tags,omitempty" json:"tags,omitempty"`
}

type PaginationParams struct {
	Page     int64
	PageSize int64
}

type QueryParams struct {
	ProjectID  string
	Pagination PaginationParams
}

func New(mongo *mongo.Client) Models {
	return Models{
		client:   mongo,
		LogEntry: LogEntry{},
		APIKey:   APIKey{},
		Settings: Settings{client: mongo},
	}
}

func (m *Models) Insert(entry LogEntry) error {
	collection := m.client.Database("logs").Collection("logs")
	_, err := collection.InsertOne(context.TODO(), LogEntry{
		ProjectID: entry.ProjectID,
		Name:      entry.Name,
		Data:      entry.Data,
		Severity:  entry.Severity,
		Tags:      entry.Tags,
		Duration:  entry.Duration,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	if err != nil {
		log.Println("Error inserting into logs:", err)
		return err
	}

	return nil
}

func (m *Models) AllLogs(p QueryParams) ([]*LogEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	collection := m.client.Database("logs").Collection("logs")
	opts := options.Find()
	opts.SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(p.Pagination.PageSize).SetSkip(p.Pagination.PageSize * (p.Pagination.Page - 1))

	cursor, err := collection.Find(context.TODO(), bson.M{"project_id": p.ProjectID}, opts)
	if err != nil {
		log.Println("Error finding docs")
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []*LogEntry

	for cursor.Next(ctx) {
		var item LogEntry

		err := cursor.Decode(&item)
		if err != nil {
			log.Println("Error decoding log into slice:", err)
			return nil, err
		}

		logs = append(logs, &item)
	}

	return logs, nil
}

func (m *Models) GetLog(id, projectID string) (*LogEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	collection := m.client.Database("logs").Collection("logs")

	docID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	var entry LogEntry
	err = collection.FindOne(ctx, bson.M{"_id": docID, "project_id": projectID}).Decode(&entry)
	if err != nil {
		return nil, err
	}

	return &entry, nil
}

// EnsureLogsIndexes creates indexes on the logs collection required for
// project-scoped queries and efficient pagination.
func (m *Models) EnsureLogsIndexes() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	coll := m.client.Database("logs").Collection("logs")
	_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "project_id", Value: 1}, {Key: "created_at", Value: -1}},
		Options: options.Index().SetName("project_id_created_at"),
	})
	if err != nil {
		return fmt.Errorf("EnsureLogsIndexes: %w", err)
	}
	return nil
}

func (m *Models) DeleteExpiredLogs(ctx context.Context, projectID string, before time.Time) (int64, error) {
	result, err := m.client.Database("logs").Collection("logs").DeleteMany(ctx, bson.M{
		"project_id": projectID,
		"created_at": bson.M{"$lt": before},
	})
	if err != nil {
		return 0, fmt.Errorf("DeleteExpiredLogs: %w", err)
	}
	return result.DeletedCount, nil
}

func (m *Models) DropLogsCollection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	collection := m.client.Database("logs").Collection("logs")

	if err := collection.Drop(ctx); err != nil {
		return err
	}

	return nil
}

func (m *Models) UpdateLog() (*mongo.UpdateResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	collection := m.client.Database("logs").Collection("logs")

	docID, err := primitive.ObjectIDFromHex(m.LogEntry.ID)
	if err != nil {
		return nil, err
	}

	result, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": docID},
		bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "name", Value: m.LogEntry.Name},
				{Key: "data", Value: m.LogEntry.Data},
				{Key: "updated_at", Value: time.Now()},
			}},
		},
	)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (m *Models) DeleteLog(id, projectID string) (*mongo.DeleteResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	collection := m.client.Database("logs").Collection("logs")

	docID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	result, err := collection.DeleteOne(ctx, bson.M{"_id": docID, "project_id": projectID})
	if err != nil {
		return nil, err
	}

	return result, nil
}

type TagCount struct {
	Tag   string `bson:"tag" json:"tag"`
	Count int    `bson:"count" json:"count"`
}

type Metrics struct {
	TotalEvents   int        `bson:"total_events" json:"total_events"`
	TotalErrors   int        `bson:"total_errors" json:"total_errors"`
	TotalCritical int        `bson:"total_critical" json:"total_critical"`
	AvgDurationMs float64    `bson:"avg_duration_ms" json:"avg_duration_ms"`
	EventsLast24h int        `bson:"events_last_24h" json:"events_last_24h"`
	ErrorsLast24h int        `bson:"errors_last_24h" json:"errors_last_24h"`
	TopTags       []TagCount `bson:"top_tags" json:"top_tags"`
}

func (m *Models) GetMetrics(projectID string) (*Metrics, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	collection := m.client.Database("logs").Collection("logs")
	since24h := time.Now().Add(-24 * time.Hour)

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"project_id": projectID}}},
		{{Key: "$facet", Value: bson.D{
			{Key: "total_events", Value: bson.A{
				bson.D{{Key: "$count", Value: "count"}},
			}},
			{Key: "total_errors", Value: bson.A{
				bson.D{{Key: "$match", Value: bson.M{"severity": bson.M{"$in": bson.A{"error", "critical"}}}}},
				bson.D{{Key: "$count", Value: "count"}},
			}},
			{Key: "total_critical", Value: bson.A{
				bson.D{{Key: "$match", Value: bson.M{"severity": "critical"}}},
				bson.D{{Key: "$count", Value: "count"}},
			}},
			{Key: "avg_duration_ms", Value: bson.A{
				bson.D{{Key: "$match", Value: bson.M{"duration": bson.M{"$gt": 0}}}},
				bson.D{{Key: "$group", Value: bson.D{
					{Key: "_id", Value: nil},
					{Key: "avg", Value: bson.D{{Key: "$avg", Value: "$duration"}}},
				}}},
			}},
			{Key: "events_last_24h", Value: bson.A{
				bson.D{{Key: "$match", Value: bson.M{"created_at": bson.M{"$gte": since24h}}}},
				bson.D{{Key: "$count", Value: "count"}},
			}},
			{Key: "errors_last_24h", Value: bson.A{
				bson.D{{Key: "$match", Value: bson.M{
					"created_at": bson.M{"$gte": since24h},
					"severity":   bson.M{"$in": bson.A{"error", "critical"}},
				}}},
				bson.D{{Key: "$count", Value: "count"}},
			}},
			{Key: "top_tags", Value: bson.A{
				bson.D{{Key: "$unwind", Value: "$tags"}},
				bson.D{{Key: "$group", Value: bson.D{
					{Key: "_id", Value: "$tags"},
					{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
				}}},
				bson.D{{Key: "$sort", Value: bson.D{{Key: "count", Value: -1}}}},
				bson.D{{Key: "$limit", Value: 5}},
				bson.D{{Key: "$project", Value: bson.D{
					{Key: "tag", Value: "$_id"},
					{Key: "count", Value: 1},
					{Key: "_id", Value: 0},
				}}},
			}},
		}}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("GetMetrics aggregate: %w", err)
	}
	defer cursor.Close(ctx)

	// $facet always returns exactly one document
	var raw []bson.M
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, fmt.Errorf("GetMetrics decode: %w", err)
	}

	result := &Metrics{}
	if len(raw) == 0 {
		return result, nil
	}

	facet := raw[0]

	if docs, ok := facet["total_events"].(bson.A); ok && len(docs) > 0 {
		result.TotalEvents = int(docs[0].(bson.M)["count"].(int32))
	}
	if docs, ok := facet["total_errors"].(bson.A); ok && len(docs) > 0 {
		result.TotalErrors = int(docs[0].(bson.M)["count"].(int32))
	}
	if docs, ok := facet["total_critical"].(bson.A); ok && len(docs) > 0 {
		result.TotalCritical = int(docs[0].(bson.M)["count"].(int32))
	}
	if docs, ok := facet["avg_duration_ms"].(bson.A); ok && len(docs) > 0 {
		result.AvgDurationMs = docs[0].(bson.M)["avg"].(float64)
	}
	if docs, ok := facet["events_last_24h"].(bson.A); ok && len(docs) > 0 {
		result.EventsLast24h = int(docs[0].(bson.M)["count"].(int32))
	}
	if docs, ok := facet["errors_last_24h"].(bson.A); ok && len(docs) > 0 {
		result.ErrorsLast24h = int(docs[0].(bson.M)["count"].(int32))
	}
	if tags, ok := facet["top_tags"].(bson.A); ok {
		for _, t := range tags {
			doc := t.(bson.M)
			result.TopTags = append(result.TopTags, TagCount{
				Tag:   doc["tag"].(string),
				Count: int(doc["count"].(int32)),
			})
		}
	}
	if result.TopTags == nil {
		result.TopTags = []TagCount{}
	}

	return result, nil
}
