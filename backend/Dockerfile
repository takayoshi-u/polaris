FROM golang:1.27-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /polaris-api ./cmd/api

FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /polaris-api /app/polaris-api

EXPOSE 8080
ENTRYPOINT ["/app/polaris-api"]

