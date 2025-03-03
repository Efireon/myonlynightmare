package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"strings"
	"time"
)

// AssetType represents the type of asset
type AssetType string

const (
	AssetTypeAudio     AssetType = "audio"
	AssetTypeTexture   AssetType = "texture"
	AssetTypeModel     AssetType = "model"
	AssetTypeShader    AssetType = "shader"
	AssetTypeAnimation AssetType = "animation"
)

// AssetMetadata represents metadata for a single asset
type AssetMetadata struct {
	ID         string             `json:"id"`
	Type       AssetType          `json:"type"`
	Tags       []string           `json:"tags"`
	Intensity  float64            `json:"intensity"`
	Conditions []string           `json:"conditions"`
	Metadata   map[string]float64 `json:"metadata"`
	CreatedAt  time.Time          `json:"created_at"`
	ModifiedAt time.Time          `json:"modified_at"`
}

// AssetCatalog represents a collection of asset metadata
type AssetCatalog struct {
	Assets map[string]*AssetMetadata `json:"assets"`
}

// NewAssetCatalog creates a new empty asset catalog
func NewAssetCatalog() *AssetCatalog {
	return &AssetCatalog{
		Assets: make(map[string]*AssetMetadata),
	}
}

// AddAsset adds a new asset to the catalog
func (ac *AssetCatalog) AddAsset(asset *AssetMetadata) error {
	if asset.ID == "" {
		return fmt.Errorf("asset ID cannot be empty")
	}

	if _, exists := ac.Assets[asset.ID]; exists {
		return fmt.Errorf("asset with ID '%s' already exists", asset.ID)
	}

	// Set timestamps if not already set
	if asset.CreatedAt.IsZero() {
		asset.CreatedAt = time.Now()
	}
	if asset.ModifiedAt.IsZero() {
		asset.ModifiedAt = asset.CreatedAt
	}

	// Convert tags to flat metadata if not already populated
	if asset.Metadata == nil {
		asset.Metadata = make(map[string]float64)
	}

	// Merge tags into metadata with default intensity if not already present
	for _, tag := range asset.Tags {
		if _, exists := asset.Metadata[tag]; !exists {
			asset.Metadata[tag] = asset.Intensity
		}
	}

	// Normalize some common tags to the hierarchical format
	normalizeMetadata(asset.Metadata)

	ac.Assets[asset.ID] = asset
	return nil
}

// UpdateAsset updates an existing asset in the catalog
func (ac *AssetCatalog) UpdateAsset(asset *AssetMetadata) error {
	if asset.ID == "" {
		return fmt.Errorf("asset ID cannot be empty")
	}

	if _, exists := ac.Assets[asset.ID]; !exists {
		return fmt.Errorf("asset with ID '%s' does not exist", asset.ID)
	}

	// Update modification timestamp
	asset.ModifiedAt = time.Now()

	// Normalize metadata
	normalizeMetadata(asset.Metadata)

	ac.Assets[asset.ID] = asset
	return nil
}

// RemoveAsset removes an asset from the catalog
func (ac *AssetCatalog) RemoveAsset(id string) error {
	if _, exists := ac.Assets[id]; !exists {
		return fmt.Errorf("asset with ID '%s' does not exist", id)
	}

	delete(ac.Assets, id)
	return nil
}

// FindAssetsByTags finds assets that match all the given tags
func (ac *AssetCatalog) FindAssetsByTags(assetType AssetType, tags []string) []*AssetMetadata {
	result := []*AssetMetadata{}

	for _, asset := range ac.Assets {
		if asset.Type != assetType {
			continue
		}

		// Check if the asset has all the required tags
		hasAllTags := true
		for _, tag := range tags {
			found := false
			for _, assetTag := range asset.Tags {
				if assetTag == tag {
					found = true
					break
				}
			}
			if !found {
				hasAllTags = false
				break
			}
		}

		if hasAllTags {
			result = append(result, asset)
		}
	}

	return result
}

// FindAssetsByConditions finds assets that match any of the given conditions
func (ac *AssetCatalog) FindAssetsByConditions(assetType AssetType, conditions []string) []*AssetMetadata {
	result := []*AssetMetadata{}

	for _, asset := range ac.Assets {
		if asset.Type != assetType {
			continue
		}

		// Check if the asset has any of the conditions
		hasAnyCondition := false
		for _, condition := range conditions {
			for _, assetCondition := range asset.Conditions {
				if assetCondition == condition {
					hasAnyCondition = true
					break
				}
			}
			if hasAnyCondition {
				break
			}
		}

		if hasAnyCondition {
			result = append(result, asset)
		}
	}

	return result
}

// FindAssetsByMetadata finds assets that match the given metadata query
func (ac *AssetCatalog) FindAssetsByMetadata(assetType AssetType, query map[string]float64, threshold float64) []*AssetMetadata {
	result := []*AssetMetadata{}

	for _, asset := range ac.Assets {
		if asset.Type != assetType {
			continue
		}

		// Calculate similarity score between query and asset metadata
		score := calculateMetadataSimilarity(query, asset.Metadata)

		if score >= threshold {
			result = append(result, asset)
		}
	}

	// Sort results by similarity score (highest first)
	sortAssetsByMetadataSimilarity(result, query)

	return result
}

// SaveToFile saves the asset catalog to a JSON file
func (ac *AssetCatalog) SaveToFile(filePath string) error {
	data, err := json.MarshalIndent(ac, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializing asset catalog: %v", err)
	}

	err = ioutil.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("error writing asset catalog file: %v", err)
	}

	return nil
}

// LoadFromFile loads the asset catalog from a JSON file
func LoadAssetCatalogFromFile(filePath string) (*AssetCatalog, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading asset catalog file: %v", err)
	}

	catalog := &AssetCatalog{
		Assets: make(map[string]*AssetMetadata),
	}

	err = json.Unmarshal(data, catalog)
	if err != nil {
		return nil, fmt.Errorf("error parsing asset catalog: %v", err)
	}

	return catalog, nil
}

// Helper functions

// normalizeMetadata converts common tags to hierarchical format
func normalizeMetadata(metadata map[string]float64) {
	// Map of common flat tags to hierarchical keys
	tagMapping := map[string]string{
		"fear":       "atmosphere.fear",
		"ominous":    "atmosphere.ominous",
		"dread":      "atmosphere.dread",
		"tension":    "atmosphere.tension",
		"distorted":  "visuals.distorted",
		"dark":       "visuals.dark",
		"glitchy":    "visuals.glitchy",
		"twisted":    "visuals.twisted",
		"fog":        "conditions.fog",
		"shadow":     "conditions.shadow",
		"silhouette": "conditions.silhouette",
		"darkness":   "conditions.darkness",
		"unnatural":  "conditions.unnatural",
	}

	// Process each metadata key
	for tag, value := range metadata {
		// Check if this is a flat tag that should be hierarchical
		if hierarchicalKey, exists := tagMapping[tag]; exists {
			// Add the hierarchical version if it doesn't already exist
			if _, hasHierarchical := metadata[hierarchicalKey]; !hasHierarchical {
				metadata[hierarchicalKey] = value
			}
		}
	}
}

// calculateMetadataSimilarity calculates a similarity score between two metadata maps
func calculateMetadataSimilarity(query, assetMetadata map[string]float64) float64 {
	if len(query) == 0 {
		return 0.0
	}

	totalScore := 0.0
	matchCount := 0

	for queryKey, queryValue := range query {
		// Look for exact matches or hierarchical prefix matches
		bestMatch := 0.0
		for assetKey, assetValue := range assetMetadata {
			similarityScore := 0.0

			// Exact match
			if queryKey == assetKey {
				// Calculate value similarity (closer values get higher scores)
				valueDiff := math.Abs(queryValue - assetValue)
				valueScore := 1.0 - math.Min(valueDiff, 1.0)
				similarityScore = 1.0 * valueScore // 100% key match * value similarity
			} else if strings.HasPrefix(queryKey, assetKey+".") || strings.HasPrefix(assetKey, queryKey+".") {
				// Hierarchical match (e.g. "atmosphere" matches with "atmosphere.fear")
				// Calculate prefix similarity by ratio of shared prefix to total length
				prefixLen := min(len(queryKey), len(assetKey))
				sharedPrefix := 0
				for i := 0; i < prefixLen; i++ {
					if queryKey[i] == assetKey[i] {
						sharedPrefix++
					} else {
						break
					}
				}
				prefixScore := float64(sharedPrefix) / float64(max(len(queryKey), len(assetKey)))

				// Calculate value similarity
				valueDiff := math.Abs(queryValue - assetValue)
				valueScore := 1.0 - math.Min(valueDiff, 1.0)

				similarityScore = prefixScore * valueScore
			}

			if similarityScore > bestMatch {
				bestMatch = similarityScore
			}
		}

		if bestMatch > 0 {
			totalScore += bestMatch
			matchCount++
		}
	}

	if matchCount == 0 {
		return 0.0
	}

	// Average match score weighted by coverage (how many query keys were matched)
	coverageRatio := float64(matchCount) / float64(len(query))
	averageScore := totalScore / float64(matchCount)

	return averageScore * coverageRatio
}

// sortAssetsByMetadataSimilarity sorts assets by their similarity to the query
func sortAssetsByMetadataSimilarity(assets []*AssetMetadata, query map[string]float64) {
	// Calculate scores once to avoid recalculating during sort
	scores := make(map[string]float64)
	for _, asset := range assets {
		scores[asset.ID] = calculateMetadataSimilarity(query, asset.Metadata)
	}

	// Sort assets by score (highest first)
	for i := 0; i < len(assets); i++ {
		for j := i + 1; j < len(assets); j++ {
			if scores[assets[i].ID] < scores[assets[j].ID] {
				assets[i], assets[j] = assets[j], assets[i]
			}
		}
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
