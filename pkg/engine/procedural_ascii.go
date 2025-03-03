package engine

import (
	"math"
	"nightmare/internal/math/noise"
)

// ASCIITexture represents a procedurally generated ASCII texture
type ASCIITexture struct {
	Width       int
	Height      int
	Data        [][]rune
	Metadata    map[string]float64
	Seed        int64
}

// ASCIIShape represents a basic shape for ASCII textures
type ASCIIShape string

const (
	ShapeCircle    ASCIIShape = "circle"
	ShapeSquare    ASCIIShape = "square"
	ShapeTriangle  ASCIIShape = "triangle"
	ShapeNoise     ASCIIShape = "noise"
	ShapeFractal   ASCIIShape = "fractal"
	ShapeDistorted ASCIIShape = "distorted"
)

// GenerateASCIITexture generates a procedural ASCII texture
func GenerateASCIITexture(width, height int, seed int64, metadata map[string]float64) *ASCIITexture {
	// Create texture
	texture := &ASCIITexture{
		Width:    width,
		Height:   height,
		Data:     make([][]rune, height),
		Metadata: metadata,
		Seed:     seed,
	}
	
	// Initialize data
	for y := 0; y < height; y++ {
		texture.Data[y] = make([]rune, width)
		for x := 0; x < width; x++ {
			texture.Data[y][x] = ' ' // Default to space
		}
	}
	
	// Create noise generator
	noiseGen := noise.NewNoiseGenerator(seed)
	
	// Determine base shape from metadata
	baseShape := determineBaseShape(metadata)
	
	// Generate base pattern
	generateBasePattern(texture, baseShape, noiseGen)
	
	// Apply modifiers based on metadata
	applyMetadataModifiers(texture, noiseGen)
	
	return texture
}

// determineBaseShape determines the base shape from metadata
func determineBaseShape(metadata map[string]float64) ASCIIShape {
	// Check if a specific shape is requested in metadata
	if val, ok := metadata["shape.type"]; ok {
		if val > 0.8 {
			return ShapeDistorted
		} else if val > 0.6 {
			return ShapeFractal
		} else if val > 0.4 {
			return ShapeNoise
		} else if val > 0.2 {
			return ShapeTriangle
		} else if val > 0.1 {
			return ShapeSquare
		} else {
			return ShapeCircle
		}
	}
	
	// Otherwise determine based on other metadata
	distortionLevel := metadata["visuals.distorted"]
	if distortionLevel > 0.7 {
		return ShapeDistorted
	}
	
	fearlevel := metadata["atmosphere.fear"]
	if fearlevel > 0.6 {
		return ShapeFractal
	}
	
	ominousLevel := metadata["atmosphere.ominous"]
	if ominousLevel > 0.5 {
		return ShapeNoise
	}
	
	// Default to circle as it's the most natural shape
	return ShapeCircle
}

// generateBasePattern generates the base pattern for the texture
func generateBasePattern(texture *ASCIITexture, shape ASCIIShape, noiseGen *noise.NoiseGenerator) {
	width := texture.Width
	height := texture.Height
	centerX := float64(width) / 2.0
	centerY := float64(height) / 2.0
	maxDist := math.Min(centerX, centerY)
	
	switch shape {
	case ShapeCircle:
		// Generate a circle pattern
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				dx := float64(x) - centerX
				dy := float64(y) - centerY
				dist := math.Sqrt(dx*dx + dy*dy)
				
				if dist < maxDist {
					// Inside circle, use dot/asterisk 
					texture.Data[y][x] = '.'
					if dist < maxDist*0.5 {
						texture.Data[y][x] = '*'
					}
				}
			}
		}
		
	case ShapeSquare:
		// Generate a square pattern
		squareSize := maxDist * 1.5
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				dx := math.Abs(float64(x) - centerX)
				dy := math.Abs(float64(y) - centerY)
				
				if dx < squareSize && dy < squareSize {
					// Inside square
					texture.Data[y][x] = '#'
					
					// Inner square
					if dx < squareSize*0.7 && dy < squareSize*0.7 {
						texture.Data[y][x] = '+'
					}
					
					// Border only (hollow square)
					if dx > squareSize*0.9 || dy > squareSize*0.9 {
						texture.Data[y][x] = '='
					}
				}
			}
		}
		
	case ShapeTriangle:
		// Generate a triangle pattern
		for y := 0; y < height; y++ {
			// Calculate width of this row of the triangle
			// Triangle gets wider as y increases
			rowWidth := (float64(y) / float64(height)) * float64(width)
			
			for x := 0; x < width; x++ {
				// Check if this point is inside the triangle
				dx := math.Abs(float64(x) - centerX)
				if dx < rowWidth/2 {
					if float64(y) > float64(height)*0.8 {
						texture.Data[y][x] = '#'
					} else if float64(y) > float64(height)*0.4 {
						texture.Data[y][x] = '/'
					} else {
						texture.Data[y][x] = '^'
					}
				}
			}
		}
		
	case ShapeNoise:
		// Generate a noise pattern
		scale := 0.1 + noiseGen.RandomFloat()*0.2
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				nx := float64(x) * scale
				ny := float64(y) * scale
				
				value := noiseGen.Perlin2D(nx, ny, texture.Seed)
				
				// Normalize to 0-1 range
				value = (value + 1.0) * 0.5
				
				// Choose character based on noise value
				if value < 0.2 {
					texture.Data[y][x] = ' '
				} else if value < 0.4 {
					texture.Data[y][x] = '.'
				} else if value < 0.6 {
					texture.Data[y][x] = ':'
				} else if value < 0.8 {
					texture.Data[y][x] = '+'
				} else {
					texture.Data[y][x] = '#'
				}
			}
		}
		
	case ShapeFractal:
		// Generate a fractal pattern (similar to a fern or lightning)
		// Start with blank canvas
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				texture.Data[y][x] = ' '
			}
		}
		
		// Draw a fractal branching pattern
		maxDepth := 5
		startX := width / 2
		startY := height - 1
		drawFractalBranch(texture, startX, startY, 0, -1, maxDepth, 1.0, noiseGen)
		
	case ShapeDistorted:
		// Generate a distorted pattern using domain warping
		scale := 0.05 + noiseGen.RandomFloat()*0.1
		warpStrength := 5.0 + noiseGen.RandomFloat()*10.0
		
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				// Base coordinates
				nx := float64(x) * scale
				ny := float64(y) * scale
				
				// Domain warping
				warpX := noiseGen.Perlin2D(nx*0.5, ny*0.5, texture.Seed)
				warpY := noiseGen.Perlin2D(nx*0.5+100, ny*0.5+100, texture.Seed)
				
				// Apply warp
				nx += warpX * warpStrength * scale
				ny += warpY * warpStrength * scale
				
				// Get noise value after warping
				value := noiseGen.Perlin2D(nx, ny, texture.Seed)
				
				// Normalize to 0-1 range
				value = (value + 1.0) * 0.5
				
				// Choose character based on noise value
				if value < 0.2 {
					texture.Data[y][x] = ' '
				} else if value < 0.3 {
					texture.Data[y][x] = '.'
				} else if value < 0.4 {
					texture.Data[y][x] = ','
				} else if value < 0.5 {
					texture.Data[y][x] = ':'
				} else if value < 0.6 {
					texture.Data[y][x] = ';'
				} else if value < 0.7 {
					texture.Data[y][x] = '+'
				} else if value < 0.8 {
					texture.Data[y][x] = '='
				} else if value < 0.9 {
					texture.Data[y][x] = '#'
				} else {
					texture.Data[y][x] = '@'
				}
			}
		}
	}
}

// drawFractalBranch recursively draws a fractal branch
func drawFractalBranch(texture *ASCIITexture, x, y, dx, dy, depth int, size float64, noiseGen *noise.NoiseGenerator) {
	if depth <= 0 || x < 0 || x >= texture.Width || y < 0 || y >= texture.Height {
		return
	}
	
	// Draw current branch
	length := int(10 * size)
	for i := 0; i < length; i++ {
		// Update position
		x += dx
		y += dy
		
		// Check bounds
		if x < 0 || x >= texture.Width || y < 0 || y >= texture.Height {
			break
		}
		
		// Draw branch character
		if depth > 3 {
			texture.Data[y][x] = '|'
		} else if depth > 1 {
			texture.Data[y][x] = '/'
		} else {
			texture.Data[y][x] = '\\'
		}
	}
	
	// Create branches
	numBranches := 2
	if depth > 2 && noiseGen.RandomFloat() < 0.5 {
		numBranches = 3
	}
	
	for i := 0; i < numBranches; i++ {
		// Calculate new direction with some randomness
		newDX := dx
		newDY := dy
		
		// Add some randomness to direction
		if noiseGen.RandomFloat() < 0.5 {
			newDX += 1
		} else {
			newDX -= 1
		}
		
		if noiseGen.RandomFloat() < 0.7 { // Bias toward growing upward
			newDY -= 1
		} else {
			newDY += 1
		}
		
		// Ensure we're not going completely horizontal or doubling back
		if newDX == 0 {
			if noiseGen.RandomFloat() < 0.5 {
				newDX = 1
			} else {
				newDX = -1
			}
		}
		if newDY >= 0 && depth > 1 { // Prevent growing downward in higher branches
			newDY = -1
		}
		
		// Normalize direction
		length := math.Sqrt(float64(newDX*newDX + newDY*newDY))
		if length != 0 {
			newDX = int(float64(newDX) / length)
			newDY = int(float64(newDY) / length)
		}
		
		// Recursively draw next branch
		newSize := size * 0.7
		drawFractalBranch(texture, x, y, newDX, newDY, depth-1, newSize, noiseGen)
	}
}

// applyMetadataModifiers applies modifiers to the texture based on metadata
func applyMetadataModifiers(texture *ASCIITexture, noiseGen *noise.NoiseGenerator) {
	width := texture.Width
	height := texture.Height
	
	// Apply distortion based on "visuals.distorted" value
	distortLevel := texture.Metadata["visuals.distorted"]
	if distortLevel > 0 {
		applyDistortion(texture, distortLevel, noiseGen)
	}
	
	// Apply glitching based on "visuals.glitchy" value
	glitchLevel := texture.Metadata["visuals.glitchy"]
	if glitchLevel > 0 {
		applyGlitching(texture, glitchLevel, noiseGen)
	}
	
	// Apply darkness based on "visuals.dark" value
	darknessLevel := texture.Metadata["visuals.dark"]
	if darknessLevel > 0 {
		applyDarkness(texture, darknessLevel, noiseGen)
	}
	
	// Apply silhouette effect based on "conditions.silhouette" value
	silhouetteLevel := texture.Metadata["conditions.silhouette"]
	if silhouetteLevel > 0 {
		applySilhouette(texture, silhouetteLevel, noiseGen)
	}
	
	// Apply unnatural modifiers for especially creepy objects
	unnaturalLevel := texture.Metadata["conditions.unnatural"]
	if unnaturalLevel > 0 {
		applyUnnatural(texture, unnaturalLevel, noiseGen)
	}
}

// applyDistortion applies distortion to the texture
func applyDistortion(texture *ASCIITexture, level float64, noiseGen *noise.NoiseGenerator) {
	width := texture.Width
	height := texture.Height
	
	// Create a copy of the original data
	originalData := make([][]rune, height)
	for y := 0; y < height; y++ {
		originalData[y] = make([]rune, width)
		copy(originalData[y], texture.Data[y])
	}
	
	// Apply distortion
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Skip empty spaces
			if originalData[y][x] == ' ' {
				continue
			}
			
			// Calculate distortion amount
			distX := int(noiseGen.RandomRange(-level*2, level*2))
			distY := int(noiseGen.RandomRange(-level*2, level*2))
			
			// Apply distortion
			newX := x + distX
			newY := y + distY
			
			// Ensure we're within bounds
			if newX >= 0 && newX < width && newY >= 0 && newY < height {
				// Apply distorted character
				// Use a different character to create a more distorted look
				originChar := originalData[y][x]
				distortedChar := originChar
				
				// Sometimes completely change the character
				if noiseGen.RandomFloat() < level*0.3 {
					distortChars := []rune{'#', '%', '&', '$', '@', '!', '*', '+', '?'}
					distortedChar = distortChars[int(noiseGen.RandomFloat()*float64(len(distortChars)))]
				}
				
				texture.Data[newY][newX] = distortedChar
			}
		}
	}
}

// applyGlitching applies glitching effects to the texture
func applyGlitching(texture *ASCIITexture, level float64, noiseGen *noise.NoiseGenerator) {
	width := texture.Width
	height := texture.Height
	
	// Number of glitch lines to add
	numGlitchLines := int(level * 5)
	
	for i := 0; i < numGlitchLines; i++ {
		// Random row for the glitch
		y := int(noiseGen.RandomFloat() * float64(height))
		
		// Random length of the glitch
		glitchLength := int(noiseGen.RandomRange(width*0.1, width*0.9))
		
		// Random start position
		startX := int(noiseGen.RandomFloat() * float64(width-glitchLength))
		
		// Random glitch character
		glitchChars := []rune{'#', '%', '&', ', '@', '!', '*', '+', '?', '-', '=', '|'}
		glitchChar := glitchChars[int(noiseGen.RandomFloat()*float64(len(glitchChars)))]
		
		// Apply glitch
		for x := startX; x < startX+glitchLength; x++ {
			if x < width {
				texture.Data[y][x] = glitchChar
			}
		}
	}
	
	// Random character replacements
	numReplacements := int(level * width * height * 0.1)
	
	for i := 0; i < numReplacements; i++ {
		x := int(noiseGen.RandomFloat() * float64(width))
		y := int(noiseGen.RandomFloat() * float64(height))
		
		if texture.Data[y][x] != ' ' {
			// Replace with a random character
			glitchChars := []rune{'#', '%', '&', ', '@', '!', '*', '+', '?', '-', '=', '|'}
			texture.Data[y][x] = glitchChars[int(noiseGen.RandomFloat()*float64(len(glitchChars)))]
		}
	}
}

// applyDarkness makes the texture darker by removing characters
func applyDarkness(texture *ASCIITexture, level float64, noiseGen *noise.NoiseGenerator) {
	width := texture.Width
	height := texture.Height
	
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Skip empty spaces
			if texture.Data[y][x] == ' ' {
				continue
			}
			
			// Chance to remove character based on darkness level
			if noiseGen.RandomFloat() < level*0.7 {
				texture.Data[y][x] = ' '
			} else {
				// Chance to change to a "lighter" character
				if noiseGen.RandomFloat() < level*0.5 {
					// Dark characters
					texture.Data[y][x] = '.'
				}
			}
		}
	}
}

// applySilhouette creates a silhouette effect
func applySilhouette(texture *ASCIITexture, level float64, noiseGen *noise.NoiseGenerator) {
	width := texture.Width
	height := texture.Height
	
	// Create a copy of the original data
	originalData := make([][]rune, height)
	for y := 0; y < height; y++ {
		originalData[y] = make([]rune, width)
		copy(originalData[y], texture.Data[y])
	}
	
	// Determine the silhouette character (usually a solid character)
	silhouetteChar := '#'
	if level > 0.7 {
		silhouetteChar = '@'
	} else if level > 0.4 {
		silhouetteChar = '%'
	}
	
	// Apply silhouette effect
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Skip empty spaces
			if originalData[y][x] == ' ' {
				continue
			}
			
			// Calculate silhouette strength based on position
			// Typically stronger on the edges
			edgeFactor := 0.0
			
			// Distance from center
			centerX := float64(width) / 2
			centerY := float64(height) / 2
			dx := math.Abs(float64(x) - centerX) / centerX
			dy := math.Abs(float64(y) - centerY) / centerY
			
			// Stronger effect on the edges
			edgeFactor = math.Max(dx, dy)
			
			// If strong silhouette level or near edge, apply silhouette
			if level*edgeFactor > 0.3 || noiseGen.RandomFloat() < level*0.3 {
				texture.Data[y][x] = silhouetteChar
			}
		}
	}
}

// applyUnnatural adds unnatural elements to make the texture creepy
func applyUnnatural(texture *ASCIITexture, level float64, noiseGen *noise.NoiseGenerator) {
	width := texture.Width
	height := texture.Height
	
	// Add random "eyes" or unsettling symbols
	numSymbols := int(level * 3)
	
	for i := 0; i < numSymbols; i++ {
		// Random position for the symbol
		x := int(noiseGen.RandomRange(width*0.2, width*0.8))
		y := int(noiseGen.RandomRange(height*0.2, height*0.8))
		
		// Different eye/symbol patterns
		eyeTypes := []string{"OO", "XX", "@@", "><", "&&"}
		selectedEye := eyeTypes[int(noiseGen.RandomFloat()*float64(len(eyeTypes)))]
		
		// Draw the symbol/eye
		if x < width-1 {
			for j := 0; j < len(selectedEye); j++ {
				if x+j < width {
					texture.Data[y][x+j] = rune(selectedEye[j])
				}
			}
		}
	}
	
	// Add distorted lines or tentacle-like structures
	numLines := int(level * 5)
	
	for i := 0; i < numLines; i++ {
		// Start at a random edge point
		startX, startY := 0, 0
		
		edge := int(noiseGen.RandomFloat() * 4) // 0=top, 1=right, 2=bottom, 3=left
		switch edge {
		case 0: // Top
			startX = int(noiseGen.RandomFloat() * float64(width))
			startY = 0
		case 1: // Right
			startX = width - 1
			startY = int(noiseGen.RandomFloat() * float64(height))
		case 2: // Bottom
			startX = int(noiseGen.RandomFloat() * float64(width))
			startY = height - 1
		case 3: // Left
			startX = 0
			startY = int(noiseGen.RandomFloat() * float64(height))
		}
		
		// Current position
		x, y := startX, startY
		
		// Random number of segments
		numSegments := 5 + int(noiseGen.RandomFloat()*10)
		
		// Tentacle character
		tentacleChar := '~'
		if noiseGen.RandomFloat() < 0.5 {
			tentacleChar = '/'
		} else if noiseGen.RandomFloat() < 0.5 {
			tentacleChar = '\\'
		}
		
		// Draw tentacle
		for j := 0; j < numSegments; j++ {
			// Move in a somewhat random direction
			dx := int(noiseGen.RandomRange(-1, 2))
			dy := int(noiseGen.RandomRange(-1, 2))
			
			// Update position
			x += dx
			y += dy
			
			// Check bounds
			if x < 0 || x >= width || y < 0 || y >= height {
				break
			}
			
			// Draw tentacle segment
			texture.Data[y][x] = tentacleChar
		}
	}
}

// GetASCIIRune returns the ASCII character at the specified texture coordinates
func (t *ASCIITexture) GetASCIIRune(u, v float64) rune {
	// Convert UV coordinates to texture coordinates
	x := int(u * float64(t.Width-1))
	y := int(v * float64(t.Height-1))
	
	// Ensure coordinates are within bounds
	if x < 0 || x >= t.Width || y < 0 || y >= t.Height {
		return ' ' // Return space for out of bounds
	}
	
	return t.Data[y][x]
}

// GetASCIIIntensity returns the intensity of the ASCII character at the specified coordinates
func (t *ASCIITexture) GetASCIIIntensity(u, v float64) float64 {
	char := t.GetASCIIRune(u, v)
	
	// Map ASCII characters to intensity values (0.0-1.0)
	switch char {
	case ' ':
		return 0.0
	case '.':
		return 0.1
	case ',':
		return 0.15
	case ':':
		return 0.2
	case ';':
		return 0.25
	case '\'':
		return 0.3
	case '"':
		return 0.35
	case '-':
		return 0.4
	case '+':
		return 0.45
	case '=':
		return 0.5
	case '*':
		return 0.6
	case '#':
		return 0.7
	case '%':
		return 0.8
	case '@':
		return 0.9
	default:
		// For other characters, estimate based on their visual density
		return 0.5
	}
}