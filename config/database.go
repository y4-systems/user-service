package config

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var MongoDB *mongo.Client

// InitMongoDB initializes MongoDB connection
func InitMongoDB() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	// Use Connect function with context and URI
	client, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		return fmt.Errorf("failed to create mongo client: %w", err)
	}

	// Verify connection with Ping
	if err = client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	MongoDB = client
	return nil
}

// GetDB returns the MongoDB database instance
func GetDB() *mongo.Database {
	dbName := os.Getenv("MONGODB_DB")
	if dbName == "" {
		dbName = "student_service"
	}
	return MongoDB.Database(dbName)
}

// DisconnectMongoDB closes MongoDB connection
func DisconnectMongoDB(ctx context.Context) error {
	if MongoDB != nil {
		return MongoDB.Disconnect(ctx)
	}
	return nil
}
