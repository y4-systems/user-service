# Student Service

A minimal Go student-management HTTP API with MongoDB persistence and Swagger documentation.

**Requirements**
- **Go**: 1.21+
- **Docker**: for container builds (optional)

**Quick Start (local)**

1. Copy or create a `.env` with these variables (do not commit secrets):

```env
MONGODB_URI=your-mongodb-uri
MONGODB_DB=student_service
SERVER_PORT=8080
SERVER_ENV=development
```

2. Run locally:

```bash
go run main.go
# or build and run
go build -o student-service .
./student-service
```

3. Open the API docs: http://localhost:8080/swagger/index.html (Swagger loads `/docs/swagger.yaml`).

**Docker (build & run)**

Build image:

```bash
docker build -t student-service:dev .
```

Run with a remote MongoDB (pass secrets at runtime):

```bash
docker run -d -p 8080:8080 \
  -e SERVER_PORT=8080 \
  -e MONGODB_URI='mongodb+srv://<user>:<pass>@cluster0...'
  --name student-service-run student-service:dev
```


**API Endpoints**
- **POST** `/auth/register` — register a student (email, password, name, phone)
- **GET** `/students/{id}` — fetch student by ID
- **PUT** `/students/{id}` — update student (email, name, phone, optional password)
- **DELETE** `/students/{id}` — delete student

Swagger UI is served at `/swagger/index.html` and the raw spec at `/docs/swagger.yaml`.


License: see LICENSE
