# Student Service 🎓

A production-ready Go microservice for student management with comprehensive security features, JWT authentication, and comprehensive testing.

## 📋 Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Requirements](#requirements)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [API Endpoints](#api-endpoints)
- [Microservice Integration](#microservice-integration)
- [Security](#security)
- [Testing](#testing)
- [Docker](#docker)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

## ✨ Features

- **JWT Authentication** - Secure token-based authentication (24h expiration)
- **Bcrypt Password Hashing** - Industry-standard password encryption
- **Rate Limiting** - Brute force protection (5 login attempts/minute per IP)
- **MongoDB Integration** - Persistent data storage with Atlas support
- **Microservice Integration** - HTTP/REST calls to Enrollment Service for combined data
- **Comprehensive Testing** - Unit tests, integration tests, and performance benchmarks
- **Swagger Documentation** - Interactive API docs with security schemes
- **CORS Support** - Cross-origin requests enabled
- **Graceful Shutdown** - Proper cleanup on termination
- **Docker Support** - Container-ready with Dockerfile

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────┐
│        API Gateway (Port 8080)                  │
│       - JWT Request Validation                  │
│       - Route Auth Headers                      │
└──────────────────┬──────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────┐
│    Student Service (Port 5001)                  │
├─────────────────────────────────────────────────┤
│ Routes:                                         │
│  POST   /auth/register           → registerH    │
│  POST   /auth/login              → loginH       │
│  GET    /auth/validate           → validateH    │
│  GET    /students/{id}           → getStudent   │
│  GET    /students/{id}/enrollments → M Call     │
│  PUT    /students/{id}           → updateH      │
│  DELETE /students/{id}           → deleteH      │
└──────────────┬──────────────────────────────────┘
               │                    │
               ▼                    ▼ (HTTP call)
        ┌────────────────┐   ┌────────────────────┐
        │  MongoDB       │   │ Enrollment Service │
        │  student_db    │   │  (Port 5003)       │
        └────────────────┘   └────────────────────┘
```

**Microservice Integration:** The Student Service calls the Enrollment Service via HTTP to fetch enrollments for the `/students/{id}/enrollments` endpoint.


## 📋 Requirements

- **Go**: 1.21 or higher
- **MongoDB**: 4.0+ (local or MongoDB Atlas)
- **Docker**: for containerization
- **curl** or **Postman**: for API testing

## 🚀 Quick Start

### 1. Clone and Setup

```bash
cd /workspaces/student-service
cp .env.example .env
```

### 2. Configure Environment

Edit `.env` with your MongoDB connection:

```env
# MongoDB Configuration
MONGODB_URI=mongodb://localhost:27017
MONGODB_DB=usersdb

# Server Configuration
SERVER_PORT=5001
SERVER_ENV=development

# Security
JWT_SECRET=your-secret-key-change-this-in-production
```

**Generate strong JWT secret:**
```bash
openssl rand -base64 32
```

### 3. Install Dependencies

```bash
go mod download
go mod verify
```

### 4. Run the Service

```bash
# Development mode (with auto-reload)
go run main.go

# Or build and run
go build -o student-service .
./student-service
```

### 5. Access Services

**Swagger UI:** http://localhost:5001/swagger/index.html  
**Health Check:** http://localhost:5001/health  
**API Base:** http://localhost:5001/api

## ⚙️ Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MONGODB_URI` | Required | MongoDB connection string |
| `MONGODB_DB` | `usersdb` | Database name |
| `SERVER_PORT` | `5001` | Service port |
| `SERVER_ENV` | `development` | Environment (development/production) |
| `JWT_SECRET` | Required | Secret for JWT signing (min 32 chars) |
| `ENROLLMENT_SERVICE_URL` | `http://localhost:5003` | Enrollment Service URL for microservice integration |

### MongoDB Setup

#### Local Development
```bash
# Using Docker
docker run -d -p 27017:27017 -e MONGO_INITDB_ROOT_USERNAME=admin \
  -e MONGO_INITDB_ROOT_PASSWORD=password mongo:latest
```

#### MongoDB Atlas (Cloud)
1. Create cluster at https://www.mongodb.com/cloud/atlas
2. Get connection string from Atlas console
3. Add string to `.env` as `MONGODB_URI`


## 🔌 API Endpoints

### Authentication Routes

#### Register Student
```http
POST /auth/register
Content-Type: application/json

{
  "email": "student@university.edu",
  "password": "securepassword123",
  "name": "John Doe",
  "phone": "1234567890"
}

Response: 201 Created
{
  "id": "507f1f77bcf86cd799439011",
  "email": "student@university.edu",
  "name": "John Doe",
  "phone": "1234567890"
}
```

#### Login
```http
POST /auth/login
Content-Type: application/json

{
  "email": "student@university.edu",
  "password": "securepassword123"
}

Response: 200 OK
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresIn": "24h",
  "user": {
    "id": "507f1f77bcf86cd799439011",
    "email": "student@university.edu",
    "name": "John Doe",
    "phone": "1234567890"
  }
}
```

#### Validate Token
```http
GET /auth/validate
Authorization: Bearer <token>

Response: 200 OK
{
  "id": "507f1f77bcf86cd799439011",
  "email": "student@university.edu",
  "name": "John Doe"
}
```

### Student Routes (Protected - Require JWT)

#### Get Student
```http
GET /students/{id}
Authorization: Bearer <token>

Response: 200 OK
{
  "id": "507f1f77bcf86cd799439011",
  "email": "student@university.edu",
  "name": "John Doe",
  "phone": "1234567890"
}
```

#### Update Student
```http
PUT /students/{id}
Authorization: Bearer <token>
Content-Type: application/json

{
  "email": "newemail@university.edu",
  "name": "Jane Doe",
  "phone": "0987654321",
  "password": "newpassword" // optional
}

Response: 200 OK
```

#### Delete Student
```http
DELETE /students/{id}
Authorization: Bearer <token>

Response: 204 No Content
```

#### Get Student with Enrollments (Microservice Integration) 🔗
```http
GET /students/{id}/enrollments
Authorization: Bearer <token>

Response: 200 OK
{
  "id": "507f1f77bcf86cd799439011",
  "email": "student@university.edu",
  "name": "John Doe",
  "phone": "1234567890",
  "enrollments": [
    {
      "_id": "60f7b2c9e1d3a4001f3e7a01",
      "student_id": "507f1f77bcf86cd799439011",
      "course_id": "C2001",
      "status": "active",
      "created_at": "2024-01-15T10:30:00Z"
    },
    {
      "_id": "60f7b2c9e1d3a4001f3e7a02",
      "student_id": "507f1f77bcf86cd799439011",
      "course_id": "C2002",
      "status": "active",
      "created_at": "2024-02-20T14:15:00Z"
    }
  ],
  "enrollment_count": 2
}
```

**Note:** This endpoint demonstrates microservice integration by calling the Enrollment Service to fetch student enrollments. See [INTEGRATION.md](INTEGRATION.md) for detailed integration documentation.

## 🔗 Microservice Integration

The Student Service integrates with the **Enrollment Service** to provide comprehensive student information including course enrollments.

**Key Features:**
- ✅ Synchronous HTTP/REST calls to Enrollment Service
- ✅ Graceful degradation if Enrollment Service unavailable
- ✅ Configurable service URL via `ENROLLMENT_SERVICE_URL` environment variable
- ✅ 10-second timeout per request for reliability
- ✅ Error logging for debugging


## 🔐 Security

### JWT (JSON Web Tokens)
- **Algorithm**: HS256
- **Expiration**: 24 hours
- **Claims**: User ID, email, name, issued-at, not-before, expires-at
- **Storage**: HTTP-only cookies or secure browser storage

### Password Security
- **Hashing**: bcrypt (cost factor 10)
- **Salting**: Automatic per password
- **Verification**: Constant-time comparison (timing attack resistant)
- **Policy**: Minimum 6 characters

### Rate Limiting
- **Limit**: 5 login attempts per minute per IP
- **Enforcement**: IP-based tracking with thread-safe map
- **Headers**: Supports X-Forwarded-For, X-Real-IP for proxied requests
- **Recovery**: Automatic after wait period
- **Response**: HTTP 429 Too Many Requests

### CORS
- **Origin**: `*` (all origins)
- **Methods**: GET, POST, PUT, DELETE, OPTIONS
- **Headers**: Content-Type, Authorization

## 🧪 Testing

### Run Unit Tests
```bash
# Fast tests only
go test -v -short ./test

# With coverage
go test -v -short -cover ./test

# All tests (including time-dependent)
go test -v ./test
```

### Run Benchmarks
```bash
go test -bench=. -benchmem ./test
```

### Interactive Demo
```bash
# Terminal 1: Start service
go run main.go

# Terminal 2: Run demo
go run test/demo/rate_limiter_demo.go
```

### Test Suite Contents
- ✅ 9 unit tests
- ✅ 2 performance benchmarks
- ✅ Integration tests
- ✅ Concurrent access tests
- ✅ Header extraction tests

See [test/README.md](test/README.md) and [test/QUICKSTART.md](test/QUICKSTART.md) for complete testing guide.

## 🐳 Docker

### Build Image
```bash
docker build -t student-service:dev .
```

### Run Container
```bash
docker run -d \
  -p 5001:5001 \
  -e MONGODB_URI='mongodb+srv://user:pass@cluster0...' \
  -e MONGODB_DB=usersdb \
  -e SERVER_PORT=5001 \
  -e JWT_SECRET='your-strong-secret' \
  --name student-service \
  student-service:dev
```

### With Docker Compose
```bash
docker-compose up -d
```

## 👨‍💻 Development

### Project Structure
```
student-service/
├── config/              # MongoDB configuration
│   └── database.go
├── types/               # Data type definitions
│   └── student.go
├── test/                # Comprehensive test suite
│   ├── rate_limiter_test.go
│   ├── demo/
│   ├── test_rate_limiter.sh
│   └── README.md
├── docs/                # Swagger/OpenAPI docs
│   ├── swagger.yaml
│   └── swagger.json
├── main.go              # Main application
├── jwt_util.go          # JWT utilities
├── rate_limiter.go      # Rate limiting implementation
├── enrollment_client.go # Enrollment Service HTTP client
├── INTEGRATION.md       # Microservice integration guide
├── .env.example         # Environment template
├── go.mod               # Go modules
├── Dockerfile           # Container configuration
└── README.md            # This file
```

### Dependencies
```bash
# Add a new dependency
go get github.com/package/name

# Update dependencies
go mod tidy

# Verify dependencies
go mod verify
```

### Code Standards
- **Format**: `gofmt` for code formatting
- **Lint**: Use `golint` to check code quality
- **Documentation**: Include comments for exported functions
- **Testing**: Write tests for new features

## 🔄 Integration with API Gateway

This service integrates with the API Gateway:

```
Client
  ↓
API Gateway (Port 8080)
  ├─ Validates JWT
  ├─ Adds X-User-ID header
  ├─ Routes to Student Service
  ↓
Student Service (Port 5001)
  ├─ Receives X-User-ID
  ├─ Processes request
  ├─ Returns response
  ↓
API Gateway (returns to client)
```

**Environment:**
```bash
STUDENT_SERVICE_URL=http://localhost:5001
```


## 🛠️ Troubleshooting

### Problem: "MONGODB_URI not set"
**Solution:** Add to `.env` or set environment variable
```bash
export MONGODB_URI="mongodb://localhost:27017"
```

### Problem: "Connection refused on port 5001"
**Solution:** Check if port is in use
```bash
lsof -i :5001
# Kill process if needed
kill -9 <PID>
```

### Problem: "Invalid JWT token"
**Solution:** Ensure:
1. Token is passed in Authorization header: `Bearer <token>`
2. JWT_SECRET matches between gateway and service
3. Token hasn't expired (24h)

### Problem: "Rate limited immediately"
**Solution:** 
1. Wait 12+ seconds for limit recovery
2. Check service logs for configuration
3. Restart service if needed

### Problem: "MongoDB connection timeout"
**Solution:**
1. Verify MONGODB_URI is correct
2. Check MongoDB is running (locally or cloud)
3. Verify network/firewall allows connection
4. Try local MongoDB: `mongodb://localhost:27017`

## 📊 Performance

**Benchmarks** (from test suite):
- GetLimiter: ~77 ns/op
- Allow (rate check): ~118 ns/op
- JWT validation: microseconds
- bcrypt hashing: ~150-250ms (intentional for security)

**Scaling**:
- Handles 1000+ concurrent connections
- Rate limiter per-IP scalable
- MongoDB queries optimized with indexing

## 📝 License

MIT - See [LICENSE](LICENSE) file for details

## 🤝 Contributing

1. Fork the repository
2. Create feature branch: `git checkout -b feature/my-feature`
3. Commit changes: `git commit -am 'Add feature'`
4. Push to branch: `git push origin feature/my-feature`
5. Create Pull Request

## 📞 Support

For issues, questions, or suggestions:
- Create an issue on GitHub
- Check existing documentation
- Review test cases for usage examples

---

