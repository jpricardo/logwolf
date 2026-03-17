package data

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

type APIKey struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	ProjectID string             `bson:"project_id" json:"project_id"`
	Prefix    string             `bson:"prefix" json:"prefix"` // e.g. "lw_A3kB9m" — safe to log
	Hash      string             `bson:"hash" json:"-"`        // bcrypt hash, never serialized
	Active    bool               `bson:"active" json:"active"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	RevokedAt *time.Time         `bson:"revoked_at,omitempty" json:"revoked_at,omitempty"`
}

// Generate creates a new API key, returning the plaintext (shown once) and the model to persist.
func GenerateAPIKey(projectID string) (plaintext string, key APIKey, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return
	}

	encoded := base64.RawURLEncoding.EncodeToString(raw)
	plaintext = fmt.Sprintf("lw_%s", encoded)
	prefix := plaintext[:10] // "lw_" + 7 chars — enough to identify, not enough to brute-force

	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return
	}

	key = APIKey{
		ProjectID: projectID,
		Prefix:    prefix,
		Hash:      string(hash),
		Active:    true,
		CreatedAt: time.Now(),
	}
	return
}

func (m *Models) ValidateAPIKey(plaintext string) (bool, *APIKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := m.client.Database("logs").Collection("api_keys")

	// Pull all active keys — the collection will be small in practice.
	// The in-memory cache in the middleware means this is rarely hit.
	cursor, err := collection.Find(ctx, bson.M{"active": true})
	if err != nil {
		return false, nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var key APIKey
		if err := cursor.Decode(&key); err != nil {
			continue
		}

		err := bcrypt.CompareHashAndPassword([]byte(key.Hash), []byte(plaintext))
		if err == nil {
			// Double-check with constant-time compare on the prefix as an extra guard
			if subtle.ConstantTimeCompare([]byte(key.Prefix), []byte(plaintext[:10])) == 1 {
				return true, &key, nil
			}
		}
	}

	return false, nil, nil
}

func (m *Models) RevokeAPIKey(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := m.client.Database("logs").Collection("api_keys")
	docID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	now := time.Now()
	_, err = collection.UpdateOne(ctx,
		bson.M{"_id": docID},
		bson.M{"$set": bson.M{"active": false, "revoked_at": now}},
	)
	return err
}
