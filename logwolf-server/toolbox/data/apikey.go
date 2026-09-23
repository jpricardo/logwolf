package data

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

const (
	apiKeyScheme = "lw_"
	// apiKeyPrefixLength is how much of a key is stored in clear as its prefix:
	// "lw_" + 7 chars — enough to identify, not enough to brute-force.
	apiKeyPrefixLength = 10
	// apiKeyLength is the length of every key GenerateAPIKey returns.
	apiKeyLength = len(apiKeyScheme) + 43 // base64.RawURLEncoding of 32 bytes
)

// ErrKeyNotFound is returned when an API key is looked up by ID but does not exist.
var ErrKeyNotFound = errors.New("api key not found")

// ErrInvalidScope is returned when a key is asked for a scope that does not exist.
var ErrInvalidScope = errors.New("invalid API key scope")

// What an API key may do on the public /logs routes.
const (
	ScopeIngest = "ingest" // POST /logs, POST /logs/batch
	ScopeRead   = "read"   // GET /logs
	ScopeDelete = "delete" // DELETE /logs
)

// AllScopes lists every scope, in the order keys store them.
var AllScopes = []string{ScopeIngest, ScopeRead, ScopeDelete}

// DefaultScopes is what a new key gets when its creator names none. Keys end
// up in browser bundles, so the default can write events but never read or
// delete them.
var DefaultScopes = []string{ScopeIngest}

type APIKey struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	ProjectID string             `bson:"project_id" json:"project_id"`
	Prefix    string             `bson:"prefix" json:"prefix"` // e.g. "lw_A3kB9m" — safe to log
	Hash      string             `bson:"hash" json:"-"`        // bcrypt hash, never serialized
	Scopes    []string           `bson:"scopes" json:"scopes"`
	// Legacy marks a key created before scopes existed. It has no scopes stored
	// and is read back with AllScopes, so it keeps the full access it always had.
	Legacy    bool       `bson:"-" json:"legacy,omitempty"`
	Active    bool       `bson:"active" json:"active"`
	CreatedAt time.Time  `bson:"created_at" json:"created_at"`
	RevokedAt *time.Time `bson:"revoked_at,omitempty" json:"revoked_at,omitempty"`
}

// HasScope reports whether the key grants scope.
func (k *APIKey) HasScope(scope string) bool {
	return slices.Contains(k.Scopes, scope)
}

// fillLegacyScopes gives a key stored before scopes existed the full access it
// had then. GenerateAPIKey never produces a key without scopes, so an empty
// list can only come from one of those.
func (k *APIKey) fillLegacyScopes() {
	if len(k.Scopes) == 0 {
		k.Scopes = slices.Clone(AllScopes)
		k.Legacy = true
	}
}

// NormalizeScopes validates scopes and returns them deduplicated, in the order
// of AllScopes. An empty list means DefaultScopes. A scope that does not exist
// is an error wrapping ErrInvalidScope.
func NormalizeScopes(scopes []string) ([]string, error) {
	if len(scopes) == 0 {
		return slices.Clone(DefaultScopes), nil
	}
	for _, s := range scopes {
		if !slices.Contains(AllScopes, s) {
			return nil, fmt.Errorf("%w: %q", ErrInvalidScope, s)
		}
	}
	var out []string
	for _, s := range AllScopes {
		if slices.Contains(scopes, s) {
			out = append(out, s)
		}
	}
	return out, nil
}

// Generate creates a new API key, returning the plaintext (shown once) and the model to persist.
// scopes goes through NormalizeScopes, so an empty list yields DefaultScopes.
func GenerateAPIKey(projectID string, scopes []string) (plaintext string, key APIKey, err error) {
	scopes, err = NormalizeScopes(scopes)
	if err != nil {
		return
	}

	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return
	}

	encoded := base64.RawURLEncoding.EncodeToString(raw)
	plaintext = apiKeyScheme + encoded
	prefix := plaintext[:apiKeyPrefixLength]

	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return
	}

	key = APIKey{
		ProjectID: projectID,
		Prefix:    prefix,
		Hash:      string(hash),
		Scopes:    scopes,
		Active:    true,
		CreatedAt: time.Now(),
	}
	return
}

// ValidateAPIKey resolves a plaintext key to the active APIKey it belongs to.
//
// It looks candidates up by prefix, and bcrypts only those — normally exactly
// one — so the cost does not grow with the number of keys across projects.
// A key GenerateAPIKey could not have produced is refused without touching the
// database.
func (m *Models) ValidateAPIKey(plaintext string) (bool, *APIKey, error) {
	if len(plaintext) != apiKeyLength || !strings.HasPrefix(plaintext, apiKeyScheme) {
		return false, nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := m.client.Database("logs").Collection("api_keys")
	cursor, err := collection.Find(ctx, bson.M{"active": true, "prefix": plaintext[:apiKeyPrefixLength]})
	if err != nil {
		return false, nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var key APIKey
		if err := cursor.Decode(&key); err != nil {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(key.Hash), []byte(plaintext)) == nil {
			key.fillLegacyScopes()
			return true, &key, nil
		}
	}

	return false, nil, cursor.Err()
}

// EnsureAPIKeyIndexes creates the index ValidateAPIKey looks keys up by.
// Safe to call on startup — CreateOne is idempotent for identical index definitions.
func (m *Models) EnsureAPIKeyIndexes() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	coll := m.client.Database("logs").Collection("api_keys")
	if _, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "prefix", Value: 1}},
		Options: options.Index().SetName("prefix"),
	}); err != nil {
		return fmt.Errorf("EnsureAPIKeyIndexes: %w", err)
	}
	return nil
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

func (m *Models) GetAPIKeyByID(id string) (*APIKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	docID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	var key APIKey
	err = m.client.Database("logs").Collection("api_keys").FindOne(ctx, bson.M{"_id": docID}).Decode(&key)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	key.fillLegacyScopes()
	return &key, nil
}

func (m *Models) ListAPIKeysByProject(projectID string) ([]APIKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := m.client.Database("logs").Collection("api_keys")
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := collection.Find(ctx, bson.M{"project_id": projectID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var keys []APIKey
	if err := cursor.All(ctx, &keys); err != nil {
		return nil, err
	}
	for i := range keys {
		keys[i].fillLegacyScopes()
	}
	return keys, nil
}

func (m *Models) SaveAPIKey(key APIKey) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := m.client.Database("logs").Collection("api_keys")
	result, err := collection.InsertOne(ctx, key)
	if err != nil {
		return err
	}

	key.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}
