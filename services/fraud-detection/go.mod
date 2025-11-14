// services/fraud-detection/go.mod
module fraud-detection

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/google/uuid v1.5.0
	github.com/lib/pq v1.10.9
	github.com/prometheus/client_golang v1.17.0
	go.mongodb.org/mongo-driver v1.13.1
	go.uber.org/zap v1.26.0
	shared v0.0.0
)

replace shared => ../../shared