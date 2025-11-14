FROM golang:1.21-alpine AS builder

WORKDIR /build

# Copy shared module first
COPY shared/ ./shared/

# Copy service code
COPY services/transaction-ledger/ ./services/transaction-ledger/

# Download dependencies
WORKDIR /build/shared
RUN go mod download

WORKDIR /build/services/transaction-ledger
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server/main.go

# Final stage
FROM alpine:3.18

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /build/services/transaction-ledger/server .

EXPOSE 8083

CMD ["./server"]