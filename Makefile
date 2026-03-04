.PHONY: help install run swagger clean

help:
	@echo "User Service API - Available Commands"
	@echo ""
	@echo "  make install   - Download Go dependencies"
	@echo "  make run       - Run the server"
	@echo "  make swagger   - Install and generate Swagger documentation"
	@echo "  make clean     - Remove generated files"
	@echo ""
	@echo "Note: Configure MongoDB Atlas in .env file"
	@echo ""

install:
	go mod download

run:
	go run main.go

swagger:
	go install github.com/swaggo/swag/cmd/swag@latest
	swag init

clean:
	rm -rf docs/
