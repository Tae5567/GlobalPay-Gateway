// services/payment-gateway/go.mod
module payment-gateway

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/google/uuid v1.5.0
	github.com/lib/pq v1.10.9
	github.com/prometheus/client_golang v1.17.0
	github.com/stripe/stripe-go/v76 v76.11.0
	go.uber.org/zap v1.26.0
	shared v0.0.0
)

replace shared => ../../shared