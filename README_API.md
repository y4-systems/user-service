# Student Service API

A simple student management API built with Go.

## Getting Started

### Install Dependencies

```bash
go mod download
```

### Install Swagger CLI (Optional - for generating Swagger docs)

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

### Generate Swagger Documentation

```bash
swag init
```

### Run the Server

```bash
go run main.go
```

The server will start on `http://localhost:8080`

## API Endpoints

### 1. Hello World
- **GET** `/`
- Returns a simple hello message

### 2. Register Student
- **POST** `/auth/register`
- Register a new user with the following JSON body:

```json
{
  "email": "student@example.com",
  "password": "securepassword",
  "name": "John Doe",
  "phone": "1234567890"
}
```

**Success Response (201):**
```json
{
  "id": 1,
  "email": "student@example.com",
  "name": "John Doe",
  "phone": "1234567890"
}
```

**Error Response (400):**
```json
{
  "error": "Missing required fields"
}
```

## Testing the API

### Using cURL

```bash
# Register a new user
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "student@example.com",
    "password": "password123",
    "name": "John Doe",
    "phone": "1234567890"
  }'
```

## API Documentation

Once the server is running and Swagger docs are generated, visit:
- `http://localhost:8080/swagger/index.html`

This provides an interactive API documentation interface.

## Project Structure

```
.
├── main.go           # Main application file
├── go.mod            # Go module file
├── types/
│   └── student.go    # Data models
└── README.md         # This file
```
