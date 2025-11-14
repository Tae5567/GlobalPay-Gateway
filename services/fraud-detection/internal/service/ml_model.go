// services/fraud-detection/internal/service/ml_model.go
package service

import (
	"context"
	"math"

	"fraud-detection/internal/models"
)

// MLModel represents a machine learning model for fraud detection
type MLModel struct {
	weights map[string]float64
	bias    float64
}

// NewMLModel creates a new ML model instance
func NewMLModel() *MLModel {
	// Initialize with pre-trained weights (simplified)
	return &MLModel{
		weights: map[string]float64{
			"amount":            0.3,
			"velocity":          0.25,
			"new_location":      0.2,
			"unusual_hour":      0.15,
			"new_device":        0.1,
		},
		bias: -0.5,
	}
}

// Predict calculates fraud probability using the model
func (m *MLModel) Predict(ctx context.Context, features map[string]float64) float64 {
	// Simple linear model: y = w1*x1 + w2*x2 + ... + bias
	var score float64 = m.bias

	for feature, value := range features {
		if weight, exists := m.weights[feature]; exists {
			score += weight * value
		}
	}

	// Apply sigmoid activation to get probability [0, 1]
	probability := m.sigmoid(score)
	
	// Convert to risk score [0, 100]
	return probability * 100
}

// sigmoid activation function
func (m *MLModel) sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

// ExtractFeatures extracts features from fraud check request
func ExtractFeatures(req *models.FraudCheckRequest, velocityCount int, isNewLocation, isUnusualHour, isNewDevice bool) map[string]float64 {
	features := make(map[string]float64)

	// Normalize amount (assuming max typical transaction is $10,000)
	features["amount"] = math.Min(req.Amount/10000.0, 1.0)

	// Velocity score (number of transactions in last hour, max 20)
	features["velocity"] = math.Min(float64(velocityCount)/20.0, 1.0)

	// Binary features
	if isNewLocation {
		features["new_location"] = 1.0
	} else {
		features["new_location"] = 0.0
	}

	if isUnusualHour {
		features["unusual_hour"] = 1.0
	} else {
		features["unusual_hour"] = 0.0
	}

	if isNewDevice {
		features["new_device"] = 1.0
	} else {
		features["new_device"] = 0.0
	}

	return features
}

// TrainModel trains the model with new data (placeholder for future implementation)
func (m *MLModel) TrainModel(ctx context.Context, trainingData []map[string]float64, labels []float64) error {
	// TODO: Implement actual training logic
	// For now, this is a placeholder for future ML integration
	// In production, you would:
	// 1. Load training data from database
	// 2. Normalize features
	// 3. Train using gradient descent or other algorithm
	// 4. Update weights and bias
	// 5. Save model to disk/database
	
	return nil
}

// LoadModel loads a pre-trained model from storage
func LoadModel(modelPath string) (*MLModel, error) {
	// TODO: Load model from file or database
	// For now, return a new model with default weights
	return NewMLModel(), nil
}

// SaveModel saves the current model to storage
func (m *MLModel) SaveModel(modelPath string) error {
	// TODO: Save model weights to file or database
	return nil
}

// EvaluateModel evaluates model performance
func (m *MLModel) EvaluateModel(ctx context.Context, testData []map[string]float64, labels []float64) map[string]float64 {
	var truePositives, falsePositives, trueNegatives, falseNegatives float64
	threshold := 0.5

	for i, features := range testData {
		prediction := m.Predict(ctx, features)
		predicted := prediction/100.0 > threshold
		actual := labels[i] > threshold

		if predicted && actual {
			truePositives++
		} else if predicted && !actual {
			falsePositives++
		} else if !predicted && !actual {
			trueNegatives++
		} else {
			falseNegatives++
		}
	}

	accuracy := (truePositives + trueNegatives) / float64(len(testData))
	precision := truePositives / (truePositives + falsePositives)
	recall := truePositives / (truePositives + falseNegatives)
	f1Score := 2 * (precision * recall) / (precision + recall)

	return map[string]float64{
		"accuracy":  accuracy,
		"precision": precision,
		"recall":    recall,
		"f1_score":  f1Score,
	}
}