FROM golang:1.21-alpine AS builder

WORKDIR /src

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

# cache deps
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# generate swagger docs
RUN go install github.com/swaggo/swag/cmd/swag@latest && \
    swag init

# build static binary
RUN go build -ldflags="-s -w" -o /app/student-service .

FROM alpine:3.18
RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/student-service /app/student-service
# Include API docs and optional .env from the build context so the runtime image can serve /docs
COPY --from=builder /src/docs /app/docs

ENV SERVER_PORT=8080
EXPOSE 8080

# non-root user
USER 1000:1000

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:${SERVER_PORT}/health || exit 1

CMD ["/app/student-service"]
