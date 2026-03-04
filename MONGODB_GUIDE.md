# MongoDB Integration Guide

## Setup Instructions

### 1. Start MongoDB (if running locally)

**Using Docker:**
```bash
docker run -d -p 27017:27017 --name mongodb mongo:latest
```

**Or using Homebrew (macOS):**
```bash
brew tap mongodb/brew
brew install mongodb-community
brew services start mongodb-community
```

**Or download from MongoDB.com:**
- Visit https://www.mongodb.com/try/download/community
- Install and run the MongoDB server

### 2. Configure Environment Variables

Create a `.env` file in the project root:
```env
MONGODB_URI=mongodb://localhost:27017
MONGODB_DB=userdb
SERVER_PORT=8080
SERVER_ENV=development
```

**For MongoDB Atlas (Cloud):**
```env
MONGODB_URI=mongodb+srv://username:password@cluster.mongodb.net/?retryWrites=true&w=majority
MONGODB_DB=userdb 
```

### 3. Run the Server

```bash
make install  # Install dependencies
make run      # Start the server
```

Or directly:
```bash
go run main.go
```

## API Usage

### Register a Student (with MongoDB persistence)

**Request:**
```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "student@example.com",
    "password": "SecurePass123",
    "name": "John Doe",
    "phone": "1234567890"
  }'
```

**Response (201 Created):**
```json
{
  "id": "65d4b2c3d9e1f2a3b4c5d6e7",
  "email": "student@example.com",
  "name": "John Doe",
  "phone": "1234567890"
}
```

The student data is now saved in MongoDB!

## Data Stored in MongoDB

Each student document in the `students` collection contains:
```json
{
  "_id": ObjectId("65d4b2c3d9e1f2a3b4c5d6e7"),
  "email": "student@example.com",
  "password": "SecurePass123",
  "name": "John Doe",
  "phone": "1234567890"
}
```

## Verify Data in MongoDB

### Using MongoDB Compass (GUI)
1. Download from https://www.mongodb.com/products/compass
2. Connect to `mongodb://localhost:27017`
3. Navigate to `student_service` → `students` collection

### Using MongoDB Shell
```bash
mongosh
use user-service
db.students.find()
```

### Using MongoDBAtlas (Cloud)
1. Visit https://account.mongodb.com
2. Navigate to your cluster
3. Use the "Browse Collections" feature in the Atlas UI

## Project Structure

```
├── main.go              # API endpoints and server setup
├── config/
│   └── database.go      # MongoDB connection and utilities
├── types/
│   └── student.go       # Data models and structs
├── .env                 # Environment variables (not committed)
├── .env.example         # Example environment file
├── go.mod               # Go module dependencies
├── go.sum               # Dependency checksums
└── Makefile             # Build and run commands
```

## Dependencies

- `go.mongodb.org/mongo-driver/v2` - MongoDB official driver for Go
- `github.com/joho/godotenv` - Environment variable loader
- `github.com/swaggo/http-swagger` - Swagger UI

## Error Handling

Common errors and solutions:

| Error | Cause | Solution |
|-------|-------|----------|
| `failed to connect to MongoDB` | MongoDB server not running | Start MongoDB service |
| `connection refused` | Wrong MONGODB_URI | Verify connection string in .env |
| `no database selected` | Invalid MONGODB_DB | Check database name in .env |

## Next Steps

- Create GET endpoints to retrieve students
- Add authentication/password hashing
- Implement error logging
- Add input validation (email duplicate check)
- Create UPDATE and DELETE endpoints

