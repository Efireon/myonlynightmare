package util

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Initialize random seed
func init() {
	rand.Seed(time.Now().UnixNano())
}

// RandomFloat returns a random float64 between min and max
func RandomFloat(min, max float64) float64 {
	return min + rand.Float64()*(max-min)
}

// RandomInt returns a random int between min and max (inclusive)
func RandomInt(min, max int) int {
	return min + rand.Intn(max-min+1)
}

// RandomBool returns a random boolean value
func RandomBool() bool {
	return rand.Intn(2) == 1
}

// RandomString returns a random string of the specified length
func RandomString(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

// RandomElement returns a random element from the slice
func RandomElement[T any](slice []T) T {
	return slice[rand.Intn(len(slice))]
}

// Lerp performs linear interpolation between a and b with t in [0,1]
func Lerp(a, b, t float64) float64 {
	return a + t*(b-a)
}

// Clamp restricts a value to be between min and max
func Clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// Map remaps a value from one range to another
func Map(value, inMin, inMax, outMin, outMax float64) float64 {
	// Calculate normalized position in input range [0,1]
	t := (value - inMin) / (inMax - inMin)
	// Clamp t to [0,1] to handle values outside the input range
	t = Clamp(t, 0, 1)
	// Apply to output range
	return outMin + t*(outMax-outMin)
}

// SmoothStep performs cubic interpolation between a and b
func SmoothStep(a, b, t float64) float64 {
	// Clamp t to [0,1]
	t = Clamp(t, 0, 1)
	// Apply cubic interpolation formula: 3t² - 2t³
	t = t * t * (3 - 2*t)
	return a + t*(b-a)
}

// Distance2D calculates the Euclidean distance between two 2D points
func Distance2D(x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	return math.Sqrt(dx*dx + dy*dy)
}

// Distance3D calculates the Euclidean distance between two 3D points
func Distance3D(x1, y1, z1, x2, y2, z2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	dz := z2 - z1
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// FileExists checks if a file exists and is not a directory
func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// DirExists checks if a directory exists
func DirExists(dirname string) bool {
	info, err := os.Stat(dirname)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// CreateDirIfNotExist creates a directory if it doesn't exist
func CreateDirIfNotExist(dir string) error {
	if !DirExists(dir) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory: %v", err)
		}
	}
	return nil
}

// ListFilesWithExt lists all files with the specified extension in a directory
func ListFilesWithExt(dir, ext string) ([]string, error) {
	var files []string

	// Make sure the extension starts with a dot
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	// Read directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// Filter files by extension
	for _, entry := range entries {
		if !entry.IsDir() {
			filename := entry.Name()
			if strings.HasSuffix(strings.ToLower(filename), strings.ToLower(ext)) {
				files = append(files, filepath.Join(dir, filename))
			}
		}
	}

	return files, nil
}

// GetFileNameWithoutExt returns the filename without extension
func GetFileNameWithoutExt(filename string) string {
	// Get the base name
	base := filepath.Base(filename)
	// Remove extension
	ext := filepath.Ext(base)
	return base[:len(base)-len(ext)]
}

// TimeTrack is a utility function to track how long a function takes to execute
// Usage: defer TimeTrack(time.Now(), "FunctionName")
func TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	fmt.Printf("%s took %s\n", name, elapsed)
}

// CalculateRollingAverage calculates a rolling average of the given data
func CalculateRollingAverage(data []float64, windowSize int) []float64 {
	if windowSize <= 0 || len(data) == 0 {
		return []float64{}
	}

	result := make([]float64, len(data))

	for i := range data {
		sum := 0.0
		count := 0

		// Calculate window bounds
		start := i - windowSize/2
		end := i + windowSize/2

		if start < 0 {
			start = 0
		}
		if end >= len(data) {
			end = len(data) - 1
		}

		// Calculate sum
		for j := start; j <= end; j++ {
			sum += data[j]
			count++
		}

		// Calculate average
		if count > 0 {
			result[i] = sum / float64(count)
		} else {
			result[i] = 0
		}
	}

	return result
}

// CalculateMedian calculates the median value of a slice
func CalculateMedian(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}

	// Create a copy of the data to avoid modifying the original
	sorted := make([]float64, len(data))
	copy(sorted, data)

	// Sort the data
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Calculate median
	middle := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[middle-1] + sorted[middle]) / 2
	}
	return sorted[middle]
}

// AngleBetween calculates the angle between two vectors in 2D space
func AngleBetween(x1, y1, x2, y2 float64) float64 {
	dot := x1*x2 + y1*y2
	len1 := math.Sqrt(x1*x1 + y1*y1)
	len2 := math.Sqrt(x2*x2 + y2*y2)

	// Avoid division by zero
	if len1 == 0 || len2 == 0 {
		return 0
	}

	cos := dot / (len1 * len2)
	// Clamp to avoid floating point errors
	cos = Clamp(cos, -1, 1)

	return math.Acos(cos)
}

// RotatePoint2D rotates a 2D point around the origin by the given angle (in radians)
func RotatePoint2D(x, y, angle float64) (float64, float64) {
	cos := math.Cos(angle)
	sin := math.Sin(angle)

	return x*cos - y*sin, x*sin + y*cos
}

// IsInside checks if a point is inside a circular area
func IsInside(px, py, cx, cy, radius float64) bool {
	dx := px - cx
	dy := py - cy
	return dx*dx+dy*dy <= radius*radius
}

// WeightedRandom returns a random index based on the weights
func WeightedRandom(weights []float64) int {
	if len(weights) == 0 {
		return -1
	}

	// Calculate total weight
	total := 0.0
	for _, w := range weights {
		total += w
	}

	// Generate random value
	value := rand.Float64() * total

	// Find index
	for i, w := range weights {
		value -= w
		if value <= 0 {
			return i
		}
	}

	// Failsafe
	return len(weights) - 1
}

// CubicBezier calculates a point on a cubic Bezier curve at time t
func CubicBezier(p0, p1, p2, p3, t float64) float64 {
	t = Clamp(t, 0, 1)

	// Bernstein basis polynomials
	b0 := (1 - t) * (1 - t) * (1 - t)
	b1 := 3 * t * (1 - t) * (1 - t)
	b2 := 3 * t * t * (1 - t)
	b3 := t * t * t

	return p0*b0 + p1*b1 + p2*b2 + p3*b3
}
