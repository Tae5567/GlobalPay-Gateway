FROM golang:1.21-alpine AS builder

WORKDIR /build

# Copy shared module first
COPY shared/ ./shared/

# Copy service code
COPY services/payment-gateway/ ./services/payment-gateway/

# Download dependencies
WORKDIR /build/shared
RUN go mod download

WORKDIR /build/services/payment-gateway
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server/main.go

# Final stage
FROM alpine:3.18

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /build/services/payment-gateway/server .

EXPOSE 8080

CMD ["./server"]