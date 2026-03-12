package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const studentIDCounterKey = "student_id_sequence"

var (
	errInvalidStudentID = errors.New("invalid student ID format")
	errStudentIDExists  = errors.New("student ID already exists")
)

var studentIDPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{2,31}$`)

type studentIDCounter struct {
	Sequence int64 `bson:"sequence"`
}

// generateNextStudentID returns IDs like STU-000001 using an atomic counter.
func generateNextStudentID(ctx context.Context, db *mongo.Database) (string, error) {
	counters := db.Collection("counters")
	update := bson.M{
		"$inc": bson.M{"sequence": 1},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var counter studentIDCounter
	if err := counters.FindOneAndUpdate(ctx, bson.M{"_id": studentIDCounterKey}, update, opts).Decode(&counter); err != nil {
		return "", err
	}

	return fmt.Sprintf("STU-%06d", counter.Sequence), nil
}

func normalizeStudentID(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

func studentIDExists(ctx context.Context, students *mongo.Collection, studentID string) (bool, error) {
	count, err := students.CountDocuments(ctx, bson.M{"studentId": studentID})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// resolveUniqueStudentID returns a valid unique ID.
// If requested is blank, it auto-generates one.
func resolveUniqueStudentID(ctx context.Context, db *mongo.Database, requested string) (string, error) {
	students := db.Collection("students")

	normalized := normalizeStudentID(requested)
	if normalized != "" {
		if !studentIDPattern.MatchString(normalized) {
			return "", errInvalidStudentID
		}
		exists, err := studentIDExists(ctx, students, normalized)
		if err != nil {
			return "", err
		}
		if exists {
			return "", errStudentIDExists
		}
		return normalized, nil
	}

	for i := 0; i < 5; i++ {
		generated, err := generateNextStudentID(ctx, db)
		if err != nil {
			return "", err
		}
		exists, err := studentIDExists(ctx, students, generated)
		if err != nil {
			return "", err
		}
		if !exists {
			return generated, nil
		}
	}

	return "", fmt.Errorf("failed to allocate a unique student ID")
}
