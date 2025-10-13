# Go build stage
FROM golang:1.25.2-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o strava-coverage ./cmd/server/main.go

# Final image
FROM alpine:latest
RUN apk add --no-cache postgresql-client
WORKDIR /app
COPY --from=builder /app/strava-coverage ./strava-coverage
COPY config ./config
COPY internal ./internal
COPY .env.example ./
EXPOSE 8080
CMD ["./strava-coverage"]
