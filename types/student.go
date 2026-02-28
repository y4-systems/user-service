package types

import "go.mongodb.org/mongo-driver/bson/primitive"

// RegisterRequest represents the student registration request
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Name     string `json:"name" binding:"required"`
	Phone    string `json:"phone" binding:"required"`
}

// LoginRequest represents the login request
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RegisterResponse represents the registration response
type RegisterResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	Token string             `json:"token"`
	User  RegisterResponse   `json:"user"`
	ExpiresIn string         `json:"expiresIn"`
}

// ValidateResponse represents the token validation response
type ValidateResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// Student represents a student in MongoDB
type Student struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email    string        `bson:"email" json:"email"`
	Password string        `bson:"password" json:"password"`
	Name     string        `bson:"name" json:"name"`
	Phone    string        `bson:"phone" json:"phone"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}
