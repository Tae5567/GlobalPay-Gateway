// services/fraud-detection/internal/service/ml_model.go
// ML model for fraud detection

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	"fraud-detection/internal/models"
)

// MLModel represents a logistic regression model for fraud detection
type MLModel struct {
	weights      map[string]float64
	bias         float64
	learningRate float64
	trained      bool
	version      string
}

// NewMLModel creates a new untrained model
func NewMLModel() *MLModel {
	return &MLModel{
		weights: map[string]float64{
			"amount":       0.0,
			"velocity":     0.0,
			"new_location": 0.0,
			"unusual_hour": 0.0,
			"new_device":   0.0,
		},
		bias:         0.0,
		learningRate: 0.01,
		trained:      false,
		version:      "1.0.0",
	}
}

// LoadPretrainedModel returns a model with good default weights
func LoadPretrainedModel() *MLModel {
	return &MLModel{
		weights: map[string]float64{
			"amount":       0.35,  // High amounts = higher fraud risk
			"velocity":     0.28,  // Many txns quickly = suspicious
			"new_location": 0.18,  // New location = moderate risk
			"unusual_hour": 0.12,  // Late night = some risk
			"new_device":   0.07,  // New device = low risk
		},
		bias:         -0.45,
		learningRate: 0.01,
		trained:      true,
		version:      "1.0.0",
	}
}

// TrainModel trains the model using gradient descent
func (m *MLModel) TrainModel(ctx context.Context, trainingData []map[string]float64, labels []float64) error {
	if len(trainingData) == 0 || len(trainingData) != len(labels) {
		return fmt.Errorf("invalid training data: got %d samples and %d labels", len(trainingData), len(labels))
	}

	epochs := 100
	batchSize := 32
	
	fmt.Printf("Starting training: %d samples, %d epochs\n", len(trainingData), epochs)

	for epoch := 0; epoch < epochs; epoch++ {
		// Shuffle data
		indices := rand.Perm(len(trainingData))
		
		epochLoss := 0.0
		correct := 0

		// Mini-batch gradient descent
		for i := 0; i < len(trainingData); i += batchSize {
			end := i + batchSize
			if end > len(trainingData) {
				end = len(trainingData)
			}

			batchLoss := 0.0
			gradients := make(map[string]float64)
			biasGradient := 0.0

			// Process batch
			for j := i; j < end; j++ {
				idx := indices[j]
				features := trainingData[idx]
				actual := labels[idx]

				// Forward pass
				prediction := m.Predict(ctx, features) / 100.0 // Convert to [0,1]
				
				// Calculate loss (binary cross-entropy)
				loss := -actual*math.Log(prediction+1e-15) - (1-actual)*math.Log(1-prediction+1e-15)
				batchLoss += loss
				epochLoss += loss

				// Check accuracy
				if (prediction > 0.5 && actual == 1.0) || (prediction <= 0.5 && actual == 0.0) {
					correct++
				}

				// Backward pass (calculate gradients)
				error := prediction - actual
				for feature, value := range features {
					gradients[feature] += error * value
				}
				biasGradient += error
			}

			// Update weights (gradient descent)
			batchLen := float64(end - i)
			for feature := range m.weights {
				if grad, exists := gradients[feature]; exists {
					m.weights[feature] -= m.learningRate * (grad / batchLen)
				}
			}
			m.bias -= m.learningRate * (biasGradient / batchLen)
		}

		// Print progress every 10 epochs
		if epoch%10 == 0 {
			accuracy := float64(correct) / float64(len(trainingData)) * 100
			avgLoss := epochLoss / float64(len(trainingData))
			fmt.Printf("Epoch %d/%d - Loss: %.4f, Accuracy: %.2f%%\n", epoch+1, epochs, avgLoss, accuracy)
		}
	}

	m.trained = true
	fmt.Println("Training complete!")
	return nil
}

// Predict calculates fraud probability
func (m *MLModel) Predict(ctx context.Context, features map[string]float64) float64 {
	score := m.bias
	for feature, value := range features {
		if weight, exists := m.weights[feature]; exists {
			score += weight * value
		}
	}
	probability := m.sigmoid(score)
	return probability * 100 // Convert to [0, 100]
}

// sigmoid activation
func (m *MLModel) sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

// ExtractFeatures creates feature vector from transaction
func ExtractFeatures(req *models.FraudCheckRequest, velocityCount int, isNewLocation, isUnusualHour, isNewDevice bool) map[string]float64 {
	features := make(map[string]float64)

	// Normalize amount [0, 1]
	features["amount"] = math.Min(req.Amount/10000.0, 1.0)

	// Normalize velocity [0, 1]
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

// SaveModel saves weights to JSON file
func (m *MLModel) SaveModel(modelPath string) error {
	data := struct {
		Weights      map[string]float64 `json:"weights"`
		Bias         float64            `json:"bias"`
		LearningRate float64            `json:"learning_rate"`
		Trained      bool               `json:"trained"`
		Version      string             `json:"version"`
		SavedAt      time.Time          `json:"saved_at"`
	}{
		Weights:      m.weights,
		Bias:         m.bias,
		LearningRate: m.learningRate,
		Trained:      m.trained,
		Version:      m.version,
		SavedAt:      time.Now(),
	}

	file, err := os.Create(modelPath)
	if err != nil {
		return fmt.Errorf("failed to create model file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode model: %w", err)
	}

	fmt.Printf("Model saved to %s\n", modelPath)
	return nil
}

// LoadModel loads weights from JSON file
func LoadModel(modelPath string) (*MLModel, error) {
	file, err := os.Open(modelPath)
	if err != nil {
		// If file doesn't exist, return pretrained model
		fmt.Println("No saved model found, using pretrained model")
		return LoadPretrainedModel(), nil
	}
	defer file.Close()

	var data struct {
		Weights      map[string]float64 `json:"weights"`
		Bias         float64            `json:"bias"`
		LearningRate float64            `json:"learning_rate"`
		Trained      bool               `json:"trained"`
		Version      string             `json:"version"`
		SavedAt      time.Time          `json:"saved_at"`
	}

	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode model: %w", err)
	}

	model := &MLModel{
		weights:      data.Weights,
		bias:         data.Bias,
		learningRate: data.LearningRate,
		trained:      data.Trained,
		version:      data.Version,
	}

	fmt.Printf("Model loaded from %s (saved: %s)\n", modelPath, data.SavedAt.Format(time.RFC3339))
	return model, nil
}

// EvaluateModel calculates metrics
func (m *MLModel) EvaluateModel(ctx context.Context, testData []map[string]float64, labels []float64) map[string]float64 {
	var truePositives, falsePositives, trueNegatives, falseNegatives float64
	threshold := 0.5

	for i, features := range testData {
		prediction := m.Predict(ctx, features) / 100.0
		predicted := prediction > threshold
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

	// Calculate metrics
	accuracy := (truePositives + trueNegatives) / float64(len(testData))
	
	precision := 0.0
	if (truePositives + falsePositives) > 0 {
		precision = truePositives / (truePositives + falsePositives)
	}
	
	recall := 0.0
	if (truePositives + falseNegatives) > 0 {
		recall = truePositives / (truePositives + falseNegatives)
	}
	
	f1Score := 0.0
	if (precision + recall) > 0 {
		f1Score = 2 * (precision * recall) / (precision + recall)
	}

	return map[string]float64{
		"accuracy":        accuracy,
		"precision":       precision,
		"recall":          recall,
		"f1_score":        f1Score,
		"true_positives":  truePositives,
		"false_positives": falsePositives,
		"true_negatives":  trueNegatives,
		"false_negatives": falseNegatives,
	}
}

// GenerateSyntheticTrainingData creates fake training data for demo
func GenerateSyntheticTrainingData(numSamples int) ([]map[string]float64, []float64) {
	rand.Seed(time.Now().UnixNano())
	
	features := make([]map[string]float64, numSamples)
	labels := make([]float64, numSamples)

	for i := 0; i < numSamples; i++ {
		// Create synthetic features
		f := make(map[string]float64)
		
		// Generate features with correlation to fraud
		if rand.Float64() < 0.2 { // 20% fraud cases
			// Fraudulent transaction patterns
			f["amount"] = rand.Float64()*0.8 + 0.2       // Higher amounts
			f["velocity"] = rand.Float64()*0.6 + 0.4     // High velocity
			f["new_location"] = rand.Float64()*0.5 + 0.5 // Often new location
			f["unusual_hour"] = rand.Float64()*0.4 + 0.6 // Unusual hours
			f["new_device"] = rand.Float64()*0.3 + 0.7   // New device
			labels[i] = 1.0 // Fraud
		} else {
			// Normal transaction patterns
			f["amount"] = rand.Float64() * 0.5           // Lower amounts
			f["velocity"] = rand.Float64() * 0.3         // Low velocity
			f["new_location"] = rand.Float64() * 0.2     // Known location
			f["unusual_hour"] = rand.Float64() * 0.3     // Normal hours
			f["new_device"] = rand.Float64() * 0.2       // Known device
			labels[i] = 0.0 // Not fraud
		}
		
		features[i] = f
	}

	return features, labels
}