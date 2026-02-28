package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"reflect"

	"github.com/joho/godotenv"
	"github.com/y4-systems/student-service/config"
	"github.com/y4-systems/student-service/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	_ "github.com/y4-systems/student-service/docs"
)

// @title Student Service API
// @version 1.0
// @description A simple student management API with MongoDB
// @BasePath /
// @schemes http https

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found, using environment variables")
	}

	// Initialize MongoDB
	if err := config.InitMongoDB(); err != nil {
		fmt.Printf("Failed to initialize MongoDB: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Successfully connected to MongoDB")

	// Setup routes with CORS middleware
	mux := http.NewServeMux()
	// Serve documentation files from ./docs at /docs/
	mux.Handle("/docs/", http.StripPrefix("/docs/", http.FileServer(http.Dir("./docs"))))
	mux.HandleFunc("/swagger.json", swaggerJSONHandler)
	mux.HandleFunc("/students/", protectedMiddleware(studentsHandler))
	mux.HandleFunc("/auth/register", registerHandler)
	mux.HandleFunc("/auth/login", loginHandler)
	mux.HandleFunc("/auth/validate", protectedMiddleware(validateHandler))
	mux.HandleFunc("/swagger/index.html", swaggerUIHandler)
	mux.HandleFunc("/swagger/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/index.html", http.StatusFound)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"message": "Hello, World!"}`)
		} else {
			http.NotFound(w, r)
		}
	})

	// Apply CORS middleware
	handler := corsMiddleware(mux)

	// Get server port
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server is running on http://localhost:%s\n", port)
	fmt.Printf("API Documentation available at http://localhost:%s/swagger/index.html\n", port)

	// Graceful shutdown handling
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		fmt.Println("Shutting down gracefully...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := config.DisconnectMongoDB(ctx); err != nil {
			fmt.Printf("Error disconnecting MongoDB: %v\n", err)
		}
		os.Exit(0)
	}()

	http.ListenAndServe(":"+port, handler)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// protectedMiddleware wraps a handler with JWT verification
func protectedMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Missing authorization header"})
			return
		}

		tokenString := ExtractTokenFromHeader(authHeader)
		if tokenString == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Invalid authorization format"})
			return
		}

		claims, err := ValidateToken(tokenString)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Invalid or expired token"})
			return
		}

		// Store claims in context for use by handlers
		ctx := r.Context()
		ctx = context.WithValue(ctx, "user", claims)
		r = r.WithContext(ctx)

		next(w, r)
	}
}

// swaggerJSONHandler serves a minimal dynamic swagger JSON
func swaggerJSONHandler(w http.ResponseWriter, r *http.Request) {
	proto := r.Header.Get("X-Forwarded-Proto")
	schemes := "[\n        \"http\",\n        \"https\"\n    ]"
	if proto != "" {
		schemes = fmt.Sprintf("[\n        \"%s\"\n    ]", proto)
	}

	swaggerJSON := fmt.Sprintf(`{
    "schemes": %s,
    "swagger": "2.0",
    "info": {
        "description": "A simple student management API with MongoDB and JWT authentication",
        "title": "Student Service API",
        "contact": {},
        "version": "1.0"
    },
    "basePath": "/",
    "securityDefinitions": {
        "BearerAuth": {
            "type": "apiKey",
            "name": "Authorization",
            "in": "header",
            "description": "JWT Authorization header using the Bearer scheme"
        }
    },
    "paths": {
        "/auth/register": {
            "post": {
                "description": "Register a new student with email, password, name, and phone",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": ["Auth"],
                "summary": "Register a new student",
                "parameters": [
                    {
                        "description": "Register Request",
                        "name": "registerRequest",
                        "in": "body",
                        "required": true,
                        "schema": {"$ref": "#/definitions/RegisterRequest"}
                    }
                ],
                "responses": {
                    "201": {"description": "Created", "schema": {"$ref": "#/definitions/RegisterResponse"}},
                    "400": {"description": "Bad Request", "schema": {"$ref": "#/definitions/ErrorResponse"}},
                    "500": {"description": "Internal Server Error", "schema": {"$ref": "#/definitions/ErrorResponse"}}
                }
            }
        },
        "/auth/login": {
            "post": {
                "description": "Login with email and password to get JWT token",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": ["Auth"],
                "summary": "Login and get JWT token",
                "parameters": [
                    {
                        "description": "Login Request",
                        "name": "loginRequest",
                        "in": "body",
                        "required": true,
                        "schema": {"$ref": "#/definitions/LoginRequest"}
                    }
                ],
                "responses": {
                    "200": {"description": "OK", "schema": {"$ref": "#/definitions/LoginResponse"}},
                    "400": {"description": "Bad Request", "schema": {"$ref": "#/definitions/ErrorResponse"}},
                    "401": {"description": "Unauthorized", "schema": {"$ref": "#/definitions/ErrorResponse"}},
                    "500": {"description": "Internal Server Error", "schema": {"$ref": "#/definitions/ErrorResponse"}}
                }
            }
        },
        "/auth/validate": {
            "get": {
                "description": "Validate JWT token and get user information",
                "produces": [
                    "application/json"
                ],
                "tags": ["Auth"],
                "summary": "Validate JWT token",
                "security": [{"BearerAuth": []}],
                "responses": {
                    "200": {"description": "OK", "schema": {"$ref": "#/definitions/ValidateResponse"}},
                    "401": {"description": "Unauthorized", "schema": {"$ref": "#/definitions/ErrorResponse"}},
                    "403": {"description": "Forbidden", "schema": {"$ref": "#/definitions/ErrorResponse"}}
                }
            }
        },
		"/students/{id}": {
			"get": {
				"description": "Get student by ID",
				"produces": ["application/json"],
				"tags": ["Students"],
				"summary": "Get student by ID",
				"security": [{"BearerAuth": []}],
				"parameters": [
					{"name": "id", "in": "path", "required": true, "type": "string", "description": "Student ID", "default": "699df7593e8c1131b613628d"}
				],
				"responses": {
					"200": {"description": "OK", "schema": {"$ref": "#/definitions/RegisterResponse"}},
					"400": {"description": "Bad Request", "schema": {"$ref": "#/definitions/ErrorResponse"}},
					"401": {"description": "Unauthorized", "schema": {"$ref": "#/definitions/ErrorResponse"}},
					"403": {"description": "Forbidden", "schema": {"$ref": "#/definitions/ErrorResponse"}},
					"404": {"description": "Not Found", "schema": {"$ref": "#/definitions/ErrorResponse"}}
				}
			},
			"put": {
				"description": "Update a student by ID",
				"consumes": ["application/json"],
				"produces": ["application/json"],
				"tags": ["Students"],
				"summary": "Update student by ID",
				"security": [{"BearerAuth": []}],
				"parameters": [
					{"name": "id", "in": "path", "required": true, "type": "string", "description": "Student ID", "default": "699df7593e8c1131b613628d"},
					{"name": "student", "in": "body", "required": true, "schema": {"$ref": "#/definitions/RegisterRequest"}}
				],
				"responses": {
					"200": {"description": "Updated", "schema": {"$ref": "#/definitions/RegisterResponse"}},
					"400": {"description": "Bad Request", "schema": {"$ref": "#/definitions/ErrorResponse"}},
					"401": {"description": "Unauthorized", "schema": {"$ref": "#/definitions/ErrorResponse"}},
					"403": {"description": "Forbidden", "schema": {"$ref": "#/definitions/ErrorResponse"}},
					"404": {"description": "Not Found", "schema": {"$ref": "#/definitions/ErrorResponse"}},
					"500": {"description": "Internal Server Error", "schema": {"$ref": "#/definitions/ErrorResponse"}}
				}
			},
			"delete": {
				"description": "Delete a student by ID",
				"produces": ["application/json"],
				"tags": ["Students"],
				"summary": "Delete student by ID",
				"security": [{"BearerAuth": []}],
				"parameters": [
					{"name": "id", "in": "path", "required": true, "type": "string", "description": "Student ID", "default": "699df7593e8c1131b613628d"}
				],
				"responses": {
					"204": {"description": "No Content"},
					"400": {"description": "Bad Request", "schema": {"$ref": "#/definitions/ErrorResponse"}},
					"401": {"description": "Unauthorized", "schema": {"$ref": "#/definitions/ErrorResponse"}},
					"403": {"description": "Forbidden", "schema": {"$ref": "#/definitions/ErrorResponse"}},
					"404": {"description": "Not Found", "schema": {"$ref": "#/definitions/ErrorResponse"}},
					"500": {"description": "Internal Server Error", "schema": {"$ref": "#/definitions/ErrorResponse"}}
				}
			}
		}
    },
    "definitions": {
        "RegisterRequest": {
			"type": "object",
			"required": ["email", "name", "password", "phone"],
			"properties": {
				"email": {"type": "string", "example": "student@example.com"},
				"name": {"type": "string", "example": "John Doe"},
				"password": {"type": "string", "minLength": 6, "example": "password123"},
				"phone": {"type": "string", "example": "1234567890"}
			},
			"example": {
				"email": "student@example.com",
				"password": "password123",
				"name": "John Doe",
				"phone": "1234567890"
			}
        },
        "LoginRequest": {
			"type": "object",
			"required": ["email", "password"],
			"properties": {
				"email": {"type": "string", "example": "student@example.com"},
				"password": {"type": "string", "example": "password123"}
			},
			"example": {
				"email": "student@example.com",
				"password": "password123"
			}
        },
        "RegisterResponse": {
            "type": "object",
            "properties": {
                "email": {"type": "string"},
                "id": {"type": "string"},
                "name": {"type": "string"},
                "phone": {"type": "string"}
            }
        },
        "LoginResponse": {
            "type": "object",
            "properties": {
                "token": {"type": "string"},
                "expiresIn": {"type": "string"},
                "user": {"$ref": "#/definitions/RegisterResponse"}
            }
        },
        "ValidateResponse": {
            "type": "object",
            "properties": {
                "id": {"type": "string"},
                "email": {"type": "string"},
                "name": {"type": "string"}
            }
        },
        "ErrorResponse": {"type": "object", "properties": {"error": {"type": "string"}}}
    }
}`, schemes)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(swaggerJSON))
}

// swaggerUIHandler serves a minimal Swagger UI that loads /swagger.json from same origin
func swaggerUIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := `<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>Swagger UI</title>
        <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist/swagger-ui.css" />
    </head>
    <body>
        <div id="swagger-ui"></div>
        <script src="https://unpkg.com/swagger-ui-dist/swagger-ui-bundle.js"></script>
        <script>
            window.onload = function() {
				const ui = SwaggerUIBundle({
					url: window.location.origin + '/docs/swagger.yaml',
                    dom_id: '#swagger-ui',
                    deepLinking: true,
                    presets: [SwaggerUIBundle.presets.apis],
                })
                window.ui = ui
            }
        </script>
    </body>
</html>`
	w.Write([]byte(html))
}

// objectIDToHex attempts to extract a hex string from different ObjectID types
func objectIDToHex(id interface{}) string {
	if id == nil {
		return ""
	}
	// try common concrete types
	if v, ok := id.(primitive.ObjectID); ok {
		return v.Hex()
	}
	// fallback: use reflection to call Hex() if available
	rv := reflect.ValueOf(id)
	m := rv.MethodByName("Hex")
	if m.IsValid() {
		res := m.Call(nil)
		if len(res) == 1 {
			if s, ok := res[0].Interface().(string); ok {
				return s
			}
		}
	}
	// last resort: use fmt
	return fmt.Sprintf("%v", id)
}

// studentsHandler handles GET and PUT for /students/{id}
func studentsHandler(w http.ResponseWriter, r *http.Request) {
	// extract id from path /students/{id}
	id := strings.TrimPrefix(r.URL.Path, "/students/")
	if id == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Missing student id"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// parse hex id
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Invalid id format"})
		return
	}

	collection := config.GetDB().Collection("students")

	switch r.Method {
		case http.MethodDelete:
			deleted, err := deleteStudent(ctx, collection, oid)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Failed to delete student"})
				return
			}
			if deleted == 0 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Student not found"})
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
	case http.MethodGet:
		var student types.Student
		err = collection.FindOne(ctx, bson.M{"_id": oid}).Decode(&student)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Student not found"})
			return
		}
		resp := types.RegisterResponse{
			ID:    student.ID.Hex(),
			Email: student.Email,
			Name:  student.Name,
			Phone: student.Phone,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)

	case http.MethodPut:
		var req types.RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Invalid request body"})
			return
		}

		// Simple validation: require name/email/phone (password optional for update)
		if req.Email == "" || req.Name == "" || req.Phone == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Missing required fields"})
			return
		}

		update := bson.M{"$set": bson.M{"email": req.Email, "name": req.Name, "phone": req.Phone}}
		if req.Password != "" {
			update["$set"].(bson.M)["password"] = req.Password
		}

		res, err := collection.UpdateOne(ctx, bson.M{"_id": oid}, update)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Failed to update student"})
			return
		}
		if res.MatchedCount == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Student not found"})
			return
		}

		// return updated document
		var student types.Student
		err = collection.FindOne(ctx, bson.M{"_id": oid}).Decode(&student)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Failed to fetch updated student"})
			return
		}
		resp := types.RegisterResponse{
			ID:    student.ID.Hex(),
			Email: student.Email,
			Name:  student.Name,
			Phone: student.Phone,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)

	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Method Not Allowed"})
	}
}

// deleteStudent performs deletion of a student document
func deleteStudent(ctx context.Context, collection interface{}, oid primitive.ObjectID) (int64, error) {
	// use the concrete collection type from config.GetDB().Collection
	col := config.GetDB().Collection("students")
	res, err := col.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

// registerHandler godoc
// @Summary Register a new student
// @Description Register a new student with email, password, name, and phone
// @Tags Auth
// @Accept json
// @Produce json
// @Param registerRequest body types.RegisterRequest true "Register Request"
// @Success 201 {object} types.RegisterResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /auth/register [post]
func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Method Not Allowed"})
		return
	}

	var req types.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Invalid request body"})
		return
	}

	// Simple validation
	if req.Email == "" || req.Password == "" || req.Name == "" || req.Phone == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Missing required fields"})
		return
	}

	// Create student object
	student := types.Student{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
		Phone:    req.Phone,
	}

	// Insert into MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := config.GetDB().Collection("students")
	result, err := collection.InsertOne(ctx, student)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Failed to register student"})
		return
	}

	// Return response with inserted ID (handle multiple possible ID types)
	idHex := objectIDToHex(result.InsertedID)
	response := types.RegisterResponse{
		ID:    idHex,
		Email: req.Email,
		Name:  req.Name,
		Phone: req.Phone,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// loginHandler godoc
// @Summary Login a student
// @Description Authenticate a student with email and password, return JWT token
// @Tags Auth
// @Accept json
// @Produce json
// @Param loginRequest body types.LoginRequest true "Login Request"
// @Success 200 {object} types.LoginResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 401 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /auth/login [post]
func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Method Not Allowed"})
		return
	}

	var req types.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Invalid request body"})
		return
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Email and password are required"})
		return
	}

	// Find student by email
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := config.GetDB().Collection("students")
	var student types.Student
	err := collection.FindOne(ctx, bson.M{"email": req.Email}).Decode(&student)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Invalid email or password"})
		return
	}

	// Verify password (simple string comparison - in production, use bcrypt)
	if student.Password != req.Password {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Invalid email or password"})
		return
	}

	// Generate JWT token
	token, err := GenerateToken(student.ID.Hex(), student.Email, student.Name)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Failed to generate token"})
		return
	}

	response := types.LoginResponse{
		Token: token,
		User: types.RegisterResponse{
			ID:    student.ID.Hex(),
			Email: student.Email,
			Name:  student.Name,
			Phone: student.Phone,
		},
		ExpiresIn: "24h",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// validateHandler godoc
// @Summary Validate JWT token
// @Description Validate a JWT token and return user information
// @Tags Auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} types.ValidateResponse
// @Failure 401 {object} types.ErrorResponse
// @Failure 403 {object} types.ErrorResponse
// @Router /auth/validate [get]
func validateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Method Not Allowed"})
		return
	}

	// Get user claims from context (set by protectedMiddleware)
	claims := r.Context().Value("user").(*JWTClaims)

	response := types.ValidateResponse{
		ID:    claims.ID,
		Email: claims.Email,
		Name:  claims.Name,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
