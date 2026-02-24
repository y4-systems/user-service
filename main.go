package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/y4-systems/student-service/types"
	httpSwagger "github.com/swaggo/http-swagger"
	_ "github.com/y4-systems/student-service/docs"
)

// @title Student Service API
// @version 1.0
// @description A simple student management API
// @host localhost:8080
// @BasePath /

func main() {
	http.HandleFunc("/", helloHandler)
	http.HandleFunc("/auth/register", registerHandler)

	// Swagger UI routes
	http.Handle("/swagger/", httpSwagger.WrapHandler)

	fmt.Println("Server is running on http://localhost:8080")
	fmt.Println("API Documentation available at http://localhost:8080/swagger/index.html")
	http.ListenAndServe(":8080", nil)
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"message": "Hello, World!"}`)
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

	// Mock registration - in a real app, you'd save to database
	response := types.RegisterResponse{
		ID:    1,
		Email: req.Email,
		Name:  req.Name,
		Phone: req.Phone,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}
