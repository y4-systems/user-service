package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/y4-systems/user-service/config"
	"github.com/y4-systems/user-service/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
)

// @title User Service API
// @version 1.0
// @description A simple student management API with MongoDB
// @BasePath /
// @schemes http https

// Global rate limiter for login endpoint
var loginRateLimiter *IPRateLimiter

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

	// Initialize rate limiter: 5 requests per minute per IP
	loginRateLimiter = NewIPRateLimiter(rate.Every(time.Minute/5), 1)
	// Cleanup old entries every hour to prevent memory leaks
	loginRateLimiter.CleanupOldEntries(time.Hour)
	fmt.Println("Rate limiter initialized: 5 login attempts per minute per IP")

	// Get server port
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "5001"
	}

	// Setup routes with CORS middleware
	mux := http.NewServeMux()
	// Serve documentation files from ./docs at /docs/
	mux.Handle("/docs/", http.StripPrefix("/docs/", http.FileServer(http.Dir("./docs"))))
	mux.HandleFunc("/swagger.json", swaggerJSONHandler)
	mux.HandleFunc("/students", protectedMiddleware(studentsHandler))
	mux.HandleFunc("/students/", protectedMiddleware(studentsHandler))
	mux.HandleFunc("/auth/register", registerHandler)
	mux.HandleFunc("/auth/login", rateLimitMiddleware(loginHandler))
	mux.HandleFunc("/auth/validate", protectedMiddleware(validateHandler))
	mux.HandleFunc("/swagger/index.html", swaggerUIHandler)
	mux.HandleFunc("/swagger/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/index.html", http.StatusFound)
	})

	// Health check endpoint (accessible without auth/DB)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","port":"%s"}`, port)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<html><body><h1>User Service</h1><a href="/swagger/index.html">Open Swagger API Docs</a></body></html>`)
		} else {
			http.NotFound(w, r)
		}
	})

	// Apply CORS middleware
	handler := corsMiddleware(mux)

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
		// Set CORS headers on all responses
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, X-Requested-With, X-User-ID")
		w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
		w.Header().Set("Access-Control-Expose-Headers", "Content-Type, X-Total-Count")

		// Handle preflight OPTIONS requests
		if r.Method == http.MethodOptions {
			w.Header().Set("Content-Type", "application/json")
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

// rateLimitMiddleware wraps a handler with rate limiting
func rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get client IP address
		ip := GetIPAddress(r)

		// Get rate limiter for this IP
		limiter := loginRateLimiter.GetLimiter(ip)

		// Check if request is allowed
		if !limiter.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(types.ErrorResponse{
				Error: "Too many login attempts. Please try again later.",
			})
			return
		}

		// Request is allowed, proceed to next handler
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
        "description": "A simple user management API with MongoDB and JWT authentication",
		"title": "User Service API",
        "contact": {},
        "version": "1.0"
    },
    "basePath": "/",
    "securityDefinitions": {
        "BearerAuth": {
            "type": "apiKey",
            "name": "Authorization",
            "in": "header",
            "description": "Enter your JWT token (Bearer prefix will be added automatically)"
        }
    },
    "paths": {
        "/auth/register": {
            "post": {
                "description": "Register a user with email, password, name, and phone",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": ["Auth"],
                "summary": "Register a new user",
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
		},
		"/students/{id}/enrollments": {
			"get": {
				"description": "Get student with course enrollments from Enrollment Service",
				"produces": ["application/json"],
				"tags": ["Students", "Microservice Integration"],
				"summary": "Get student with enrollments",
				"security": [{"BearerAuth": []}],
				"parameters": [
					{"name": "id", "in": "path", "required": true, "type": "string", "description": "Student ID", "default": "699df7593e8c1131b613628d"}
				],
				"responses": {
					"200": {"description": "OK", "schema": {"$ref": "#/definitions/StudentWithEnrollments"}},
					"400": {"description": "Bad Request", "schema": {"$ref": "#/definitions/ErrorResponse"}},
					"401": {"description": "Unauthorized", "schema": {"$ref": "#/definitions/ErrorResponse"}},
					"403": {"description": "Forbidden", "schema": {"$ref": "#/definitions/ErrorResponse"}},
					"404": {"description": "Not Found", "schema": {"$ref": "#/definitions/ErrorResponse"}}
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
                "name": {"type": "string"},
                "role": {"type": "string"}
            }
        },
        "EnrollmentRecord": {
            "type": "object",
            "properties": {
                "_id": {"type": "string", "example": "60f7b2c9e1d3a4001f3e7a01"},
                "student_id": {"type": "string", "example": "507f1f77bcf86cd799439011"},
                "course_id": {"type": "string", "example": "C2001"},
                "status": {"type": "string", "enum": ["active", "completed", "dropped"], "example": "active"},
                "created_at": {"type": "string", "example": "2024-01-15T10:30:00Z"},
                "updated_at": {"type": "string", "example": "2024-01-15T10:30:00Z"}
            }
        },
        "StudentWithEnrollments": {
            "type": "object",
            "properties": {
                "id": {"type": "string"},
                "email": {"type": "string"},
                "name": {"type": "string"},
                "phone": {"type": "string"},
                "enrollments": {
                    "type": "array",
                    "items": {"$ref": "#/definitions/EnrollmentRecord"}
                },
                "enrollment_count": {"type": "integer"}
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
		<title>User Service Swagger UI</title>
        <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist/swagger-ui.css" />
		<style>
			body {
				margin: 0;
			}

			.theme-toggle {
				position: fixed;
				top: 12px;
				right: 12px;
				z-index: 10000;
				border: 1px solid #3b4151;
				border-radius: 6px;
				padding: 8px 12px;
				cursor: pointer;
				font-size: 14px;
				background: #ffffff;
				color: #3b4151;
			}

			body.dark-mode {
				background: #111827;
			}

			body.dark-mode .theme-toggle {
				background: #1f2937;
				color: #e5e7eb;
				border-color: #374151;
			}

			body.dark-mode .swagger-ui,
			body.dark-mode .swagger-ui .wrapper,
			body.dark-mode .swagger-ui .information-container,
			body.dark-mode .swagger-ui .scheme-container,
			body.dark-mode .swagger-ui .opblock,
			body.dark-mode .swagger-ui .opblock .opblock-summary,
			body.dark-mode .swagger-ui .opblock-section,
			body.dark-mode .swagger-ui .opblock-section-header,
			body.dark-mode .swagger-ui .responses-inner,
			body.dark-mode .swagger-ui .responses-table,
			body.dark-mode .swagger-ui table,
			body.dark-mode .swagger-ui .model-box,
			body.dark-mode .swagger-ui .tab,
			body.dark-mode .swagger-ui .highlight-code,
			body.dark-mode .swagger-ui .microlight,
			body.dark-mode .swagger-ui .opblock-body pre,
			body.dark-mode .swagger-ui textarea,
			body.dark-mode .swagger-ui input,
			body.dark-mode .swagger-ui select {
				background: #1f2937 !important;
				color: #e5e7eb !important;
				border-color: #374151 !important;
			}

			body.dark-mode .swagger-ui .topbar {
				background-color: #111827 !important;
			}

			body.dark-mode .swagger-ui,
			body.dark-mode .swagger-ui .info .title,
			body.dark-mode .swagger-ui .info p,
			body.dark-mode .swagger-ui .info li,
			body.dark-mode .swagger-ui .opblock-tag,
			body.dark-mode .swagger-ui .opblock-summary-description,
			body.dark-mode .swagger-ui .opblock-summary-path,
			body.dark-mode .swagger-ui .opblock-summary-method,
			body.dark-mode .swagger-ui .opblock-section-header h4,
			body.dark-mode .swagger-ui .opblock-section-header label,
			body.dark-mode .swagger-ui .parameter__name,
			body.dark-mode .swagger-ui .parameter__type,
			body.dark-mode .swagger-ui .parameter__deprecated,
			body.dark-mode .swagger-ui .parameter__in,
			body.dark-mode .swagger-ui .response-col_status,
			body.dark-mode .swagger-ui .response-col_description,
			body.dark-mode .swagger-ui .response-col_description__inner,
			body.dark-mode .swagger-ui .model-title,
			body.dark-mode .swagger-ui .model,
			body.dark-mode .swagger-ui .prop-name,
			body.dark-mode .swagger-ui .prop-type,
			body.dark-mode .swagger-ui .btn,
			body.dark-mode .swagger-ui label,
			body.dark-mode .swagger-ui h1,
			body.dark-mode .swagger-ui h2,
			body.dark-mode .swagger-ui h3,
			body.dark-mode .swagger-ui h4,
			body.dark-mode .swagger-ui h5,
			body.dark-mode .swagger-ui h6,
			body.dark-mode .swagger-ui span,
			body.dark-mode .swagger-ui td,
			body.dark-mode .swagger-ui th,
			body.dark-mode .swagger-ui p,
			body.dark-mode .swagger-ui a {
				color: #e5e7eb !important;
			}

			body.dark-mode .swagger-ui .opblock.opblock-get,
			body.dark-mode .swagger-ui .opblock.opblock-post,
			body.dark-mode .swagger-ui .opblock.opblock-put,
			body.dark-mode .swagger-ui .opblock.opblock-delete,
			body.dark-mode .swagger-ui .opblock.opblock-patch {
				border-color: #374151 !important;
			}

			body.dark-mode .swagger-ui .microlight span,
			body.dark-mode .swagger-ui .highlight-code span,
			body.dark-mode .swagger-ui pre,
			body.dark-mode .swagger-ui code,
			body.dark-mode .swagger-ui .markdown code {
				color: #e5e7eb !important;
			}

			body.dark-mode .swagger-ui .btn.execute {
				border-color: #60a5fa !important;
				color: #60a5fa !important;
				background: #1f2937 !important;
			}

			body.dark-mode .swagger-ui .try-out__btn,
			body.dark-mode .swagger-ui .btn.try-out__btn,
			body.dark-mode .swagger-ui .btn.cancel,
			body.dark-mode .swagger-ui .btn.clear,
			body.dark-mode .swagger-ui .btn.authorize,
			body.dark-mode .swagger-ui .opblock-control-arrow {
				color: #e5e7eb !important;
				border-color: #4b5563 !important;
				background: #1f2937 !important;
			}

			body.dark-mode .swagger-ui .parameters-col_description input::placeholder,
			body.dark-mode .swagger-ui textarea::placeholder {
				color: #9ca3af !important;
			}

			body.dark-mode .swagger-ui .dialog-ux .modal-ux,
			body.dark-mode .swagger-ui .dialog-ux .modal-ux-content,
			body.dark-mode .swagger-ui .dialog-ux .modal-ux-header,
			body.dark-mode .swagger-ui .dialog-ux .modal-ux-body,
			body.dark-mode .swagger-ui .dialog-ux .auth-container,
			body.dark-mode .swagger-ui .dialog-ux .scope-def {
				background: #1f2937 !important;
				color: #e5e7eb !important;
				border-color: #374151 !important;
			}

			body.dark-mode .swagger-ui .dialog-ux .modal-ux-header h3,
			body.dark-mode .swagger-ui .dialog-ux .auth-container h4,
			body.dark-mode .swagger-ui .dialog-ux .auth-container p,
			body.dark-mode .swagger-ui .dialog-ux label,
			body.dark-mode .swagger-ui .dialog-ux .scopes h2,
			body.dark-mode .swagger-ui .dialog-ux .scope-def,
			body.dark-mode .swagger-ui .dialog-ux .scope-def span,
			body.dark-mode .swagger-ui .dialog-ux .errors-wrapper,
			body.dark-mode .swagger-ui .dialog-ux .markdown p,
			body.dark-mode .swagger-ui .dialog-ux .markdown code {
				color: #e5e7eb !important;
			}

			body.dark-mode .swagger-ui .dialog-ux input[type=text],
			body.dark-mode .swagger-ui .dialog-ux input[type=password] {
				background: #111827 !important;
				color: #e5e7eb !important;
				border: 1px solid #4b5563 !important;
			}

			body.dark-mode .swagger-ui .dialog-ux .btn,
			body.dark-mode .swagger-ui .dialog-ux .btn.modal-btn {
				background: #1f2937 !important;
				color: #e5e7eb !important;
				border-color: #4b5563 !important;
			}

			body.dark-mode .swagger-ui .dialog-ux .btn.authorize {
				color: #60a5fa !important;
				border-color: #60a5fa !important;
			}

			body.dark-mode .swagger-ui .dialog-ux .close-modal {
				color: #e5e7eb !important;
			}

			body.dark-mode .swagger-ui a {
				color: #93c5fd !important;
			}
		</style>
    </head>
    <body>
		<button id="theme-toggle" class="theme-toggle" type="button">🌙 Dark</button>
        <div id="swagger-ui"></div>
        <script src="https://unpkg.com/swagger-ui-dist/swagger-ui-bundle.js"></script>
        <script>
			function applyTheme(theme) {
				const dark = theme === 'dark';
				document.body.classList.toggle('dark-mode', dark);
				const btn = document.getElementById('theme-toggle');
				if (btn) {
					btn.textContent = dark ? '☀️ Light' : '🌙 Dark';
				}
			}

			function getInitialTheme() {
				const saved = localStorage.getItem('swagger-theme');
				if (saved === 'dark' || saved === 'light') return saved;
				return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
			}

            window.onload = function() {
				let theme = getInitialTheme();
				applyTheme(theme);

				document.getElementById('theme-toggle').addEventListener('click', function() {
					theme = theme === 'dark' ? 'light' : 'dark';
					localStorage.setItem('swagger-theme', theme);
					applyTheme(theme);
				});

				const ui = SwaggerUIBundle({
					url: window.location.origin + '/swagger.json',
                    dom_id: '#swagger-ui',
                    deepLinking: true,
                    presets: [SwaggerUIBundle.presets.apis],
                    requestInterceptor: (request) => {
                        // Automatically prepend 'Bearer ' to Authorization header if not already present
                        if (request.headers.Authorization && !request.headers.Authorization.startsWith('Bearer ')) {
                            request.headers.Authorization = 'Bearer ' + request.headers.Authorization;
                        }
                        return request;
                    }
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

// studentsHandler handles GET /students/ (list) and GET/PUT/DELETE /students/{id}
func studentsHandler(w http.ResponseWriter, r *http.Request) {
	// extract id from path /students/{id}
	path := r.URL.Path
	// Strip /students or /students/ prefix, then remove leading slash to get bare id
	id := strings.TrimPrefix(strings.TrimPrefix(path, "/students"), "/")
	// Normalise: treat "/" (trailing-slash-only) as empty (list request)
	if id == "/" {
		id = ""
	}

	// No ID supplied → list all students (admin only)
	if id == "" {
		if r.Method != http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Method Not Allowed"})
			return
		}
		claims, ok := r.Context().Value("user").(*JWTClaims)
		if !ok || claims == nil || claims.Role != "admin" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Admin access required"})
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		collection := config.GetDB().Collection("students")
		cursor, err := collection.Find(ctx, bson.M{})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Failed to list students"})
			return
		}
		defer cursor.Close(ctx)
		var results []types.RegisterResponse
		for cursor.Next(ctx) {
			var s types.Student
			if err := cursor.Decode(&s); err != nil {
				continue
			}
			results = append(results, types.RegisterResponse{
				ID:        s.ID.Hex(),
				StudentID: s.StudentID,
				Email:     s.Email,
				Name:      s.Name,
				Phone:     s.Phone,
			})
		}
		if results == nil {
			results = []types.RegisterResponse{}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(results)
		return
	}

	// Check if this is an enrollments request: /students/{id}/enrollments
	if strings.Contains(id, "/enrollments") {
		studentEnrollmentsHandler(w, r)
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
			ID:        student.ID.Hex(),
			StudentID: student.StudentID,
			Email:     student.Email,
			Name:      student.Name,
			Phone:     student.Phone,
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
			// Hash the new password before storing
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Failed to process password"})
				return
			}
			update["$set"].(bson.M)["password"] = string(hashedPassword)
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
			ID:        student.ID.Hex(),
			StudentID: student.StudentID,
			Email:     student.Email,
			Name:      student.Name,
			Phone:     student.Phone,
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
// @Summary Register a new user Available authorizations

// @Description Register a new user with email, password, name, and phone
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

	// Hash password using bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Failed to process password"})
		return
	}

	// Validate and set role (default to "student" if not provided)
	role := req.Role
	if role == "" {
		role = "student"
	}
	// Validate role value
	if role != "student" && role != "admin" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Invalid role. Must be 'student' or 'admin'"})
		return
	}

	// Insert into MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	studentID := ""
	if role == "student" {
		studentID, err = resolveUniqueStudentID(ctx, config.GetDB(), req.StudentID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case errors.Is(err, errInvalidStudentID):
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Invalid student ID format. Use 3-32 chars: letters, numbers, hyphen."})
			case errors.Is(err, errStudentIDExists):
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Student ID already exists"})
			default:
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Failed to generate student ID"})
			}
			return
		}
	}

	// Create student object
	student := types.Student{
		Email:     req.Email,
		Password:  string(hashedPassword),
		Name:      req.Name,
		Phone:     req.Phone,
		Role:      role,
		StudentID: studentID,
	}

	// Determine collection based on role
	collectionName := "students"
	if role == "admin" {
		collectionName = "admins"
	}

	collection := config.GetDB().Collection(collectionName)
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
		ID:        idHex,
		StudentID: studentID,
		Email:     req.Email,
		Name:      req.Name,
		Phone:     req.Phone,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// loginHandler godoc
// @Summary Login
// @Description Authenticate a USER with email and password, return JWT token
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

	// Find student by email - check both students and admins collections
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var student types.Student

	// Try to find in students collection first
	studentsCollection := config.GetDB().Collection("students")
	err := studentsCollection.FindOne(ctx, bson.M{"email": req.Email}).Decode(&student)

	// If not found in students, try admins collection
	if err != nil {
		adminsCollection := config.GetDB().Collection("admins")
		err = adminsCollection.FindOne(ctx, bson.M{"email": req.Email}).Decode(&student)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Invalid email or password"})
			return
		}
	}

	// Verify password using bcrypt
	err = bcrypt.CompareHashAndPassword([]byte(student.Password), []byte(req.Password))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Invalid email or password"})
		return
	}

	// Generate JWT token with role from stored document
	token, err := GenerateToken(student.ID.Hex(), student.Email, student.Name, student.Role)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Failed to generate token"})
		return
	}

	response := types.LoginResponse{
		Token: token,
		User: types.RegisterResponse{
			ID:        student.ID.Hex(),
			StudentID: student.StudentID,
			Email:     student.Email,
			Name:      student.Name,
			Phone:     student.Phone,
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
		Role:  claims.Role,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// studentEnrollmentsHandler godoc
// @Summary Get student enrollments
// @Description Get all enrollments for a specific student by calling the Enrollment Service (microservice integration)
// @Tags Students
// @Security BearerAuth
// @Produce json
// @Param id path string true "Student ID" example:"507f1f77bcf86cd799439011"
// @Success 200 {object} types.StudentWithEnrollments
// @Failure 400 {object} types.ErrorResponse
// @Failure 401 {object} types.ErrorResponse
// @Failure 403 {object} types.ErrorResponse
// @Failure 404 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /students/{id}/enrollments [get]
func studentEnrollmentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Method Not Allowed"})
		return
	}

	// Extract student ID from path: /students/{id}/enrollments
	path := r.URL.Path
	parts := strings.Split(path, "/")
	// Path format: /students/{id}/enrollments
	// parts[0] = "", parts[1] = "students", parts[2] = {id}, parts[3] = "enrollments"
	if len(parts) < 3 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Missing student id"})
		return
	}

	studentID := parts[2]
	if studentID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Missing student id"})
		return
	}

	// Validate student ID format
	oid, err := primitive.ObjectIDFromHex(studentID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Invalid student id format"})
		return
	}

	// Get student from MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := config.GetDB().Collection("students")
	var student types.Student
	err = collection.FindOne(ctx, bson.M{"_id": oid}).Decode(&student)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(types.ErrorResponse{Error: "Student not found"})
		return
	}

	// Call Enrollment Service to get student's enrollments
	enrollmentClient := NewEnrollmentClient()
	studentEnrollments, err := enrollmentClient.GetStudentEnrollments(studentID)
	if err != nil {
		fmt.Printf("Failed to fetch enrollments from Enrollment Service: %v\n", err)
		// Return student data with empty enrollments list instead of failing
		response := types.StudentWithEnrollments{
			ID:              student.ID.Hex(),
			Email:           student.Email,
			Name:            student.Name,
			Phone:           student.Phone,
			Enrollments:     []types.EnrollmentRecord{},
			EnrollmentCount: 0,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Convert to response format
	enrollmentRecords := make([]types.EnrollmentRecord, len(studentEnrollments.Enrollments))
	for i, enrollment := range studentEnrollments.Enrollments {
		enrollmentRecords[i] = types.EnrollmentRecord{
			ID:        enrollment.ID,
			StudentID: enrollment.StudentID,
			CourseID:  enrollment.CourseID,
			Status:    enrollment.Status,
			CreatedAt: enrollment.CreatedAt.String(),
			UpdatedAt: enrollment.UpdatedAt.String(),
		}
	}

	response := types.StudentWithEnrollments{
		ID:              student.ID.Hex(),
		Email:           student.Email,
		Name:            student.Name,
		Phone:           student.Phone,
		Enrollments:     enrollmentRecords,
		EnrollmentCount: len(enrollmentRecords),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
