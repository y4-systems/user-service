package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/y4-systems/student-service/config"
	"github.com/y4-systems/student-service/types"
	"go.mongodb.org/mongo-driver/v2/bson"
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
	mux.HandleFunc("/swagger.json", swaggerJSONHandler)
	mux.HandleFunc("/auth/register", registerHandler)
	// Serve a lightweight Swagger UI that always loads the swagger JSON from the same origin.
	// This avoids hard-coded hosts and works behind proxies (Codespaces, ngrok, etc.).
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

// swaggerJSONHandler serves the swagger.json with the correct host
func swaggerJSONHandler(w http.ResponseWriter, r *http.Request) {
	// Prefer X-Forwarded headers when behind a proxy (Codespaces, ngrok, etc.)
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}

	proto := r.Header.Get("X-Forwarded-Proto")
	schemes := "[\n        \"http\",\n        \"https\"\n    ]"
	if proto != "" {
		// use only the forwarded proto to avoid mixed-scheme CORS issues
		schemes = fmt.Sprintf("[\n        \"%s\"\n    ]", proto)
	}

	swaggerJSON := fmt.Sprintf(`{
	"schemes": %s,
	"swagger": "2.0",
    "info": {
        "description": "A simple student management API with MongoDB",
        "title": "Student Service API",
        "contact": {},
        "version": "1.0"
    },
	"basePath": "/",
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
                "tags": [
                    "Auth"
                ],
                "summary": "Register a new student",
                "parameters": [
                    {
                        "description": "Register Request",
                        "name": "registerRequest",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/RegisterRequest"
                        }
                    }
                ],
                "responses": {
                    "201": {
                        "description": "Created",
                        "schema": {
                            "$ref": "#/definitions/RegisterResponse"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/ErrorResponse"
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "RegisterRequest": {
            "type": "object",
            "properties": {
                "email": {
                    "type": "string"
                },
                "name": {
                    "type": "string"
                },
                "password": {
                    "type": "string"
                },
                "phone": {
                    "type": "string"
                }
            }
        },
        "RegisterResponse": {
            "type": "object",
            "properties": {
                "email": {
                    "type": "string"
                },
                "id": {
                    "type": "string"
                },
                "name": {
                    "type": "string"
                },
                "phone": {
                    "type": "string"
                }
            }
        },
        "ErrorResponse": {
            "type": "object",
            "properties": {
                "error": {
                    "type": "string"
                }
            }
        }
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
					url: window.location.origin + '/swagger.json',
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

	// Return response with inserted ID
	objID := result.InsertedID.(bson.ObjectID)
	response := types.RegisterResponse{
		ID:    objID.Hex(),
		Email: req.Email,
		Name:  req.Name,
		Phone: req.Phone,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}
