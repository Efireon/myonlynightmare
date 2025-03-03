package noise

import (
	"math"
	"math/rand"
)

// NoiseGenerator is a utility for generating different types of noise
type NoiseGenerator struct {
	rng *rand.Rand


// Ridge2D generates 2D ridge noise (useful for terrain ridges/mountains)
func (ng *NoiseGenerator) Ridge2D(x, y float64, seed int64) float64 {
	// Get raw perlin value
	n := ng.Perlin2D(x, y, seed)
	
	// Take absolute value to create ridges
	n = math.Abs(n)
	
	// Invert to turn ridges into valleys
	n = 1.0 - n
	
	// Sharpen the ridges with a power function
	n = math.Pow(n, 2)
	
	return n
}

// FBM2D generates 2D Fractal Brownian Motion noise
func (ng *NoiseGenerator) FBM2D(x, y float64, octaves int, lacunarity, gain float64, seed int64) float64 {
	result := 0.0
	amplitude := 1.0
	frequency := 1.0
	max := 0.0
	
	for i := 0; i < octaves; i++ {
		result += ng.Perlin2D(x*frequency, y*frequency, seed+int64(i)) * amplitude
		max += amplitude
		amplitude *= gain
		frequency *= lacunarity
	}
	
	// Normalize
	return result / max
}

// Cellular2D generates 2D Worley/Cellular noise
func (ng *NoiseGenerator) Cellular2D(x, y float64, seed int64) float64 {
	// Find cell coordinates
	ix := int(math.Floor(x))
	iy := int(math.Floor(y))
	
	minDist := 1.0
	
	// Check surrounding cells (including center)
	for nx := -1; nx <= 1; nx++ {
		for ny := -1; ny <= 1; ny++ {
			// Calculate cell coordinates
			cx := ix + nx
			cy := iy + ny
			
			// Generate a random point within this cell using the hash
			h := hash(cx, cy, 0, int(seed))
			px := float64(cx) + hashToFloat(h)
			h = hash(cx, cy, 1, int(seed))
			py := float64(cy) + hashToFloat(h)
			
			// Calculate distance to this point
			dx := px - x
			dy := py - y
			dist := math.Sqrt(dx*dx + dy*dy)
			
			// Keep track of minimum distance
			minDist = math.Min(minDist, dist)
		}
	}
	
	return minDist
}

// Simplex2D generates 2D Simplex noise
func (ng *NoiseGenerator) Simplex2D(x, y float64, seed int64) float64 {
	// Skew input space to determine simplex cell
	const F2 = 0.366025404 // 0.5 * (sqrt(3) - 1)
	const G2 = 0.211324865 // (3 - sqrt(3)) / 6
	
	s := (x + y) * F2
	i := math.Floor(x + s)
	j := math.Floor(y + s)
	
	t := (i + j) * G2
	X0 := i - t
	Y0 := j - t
	x0 := x - X0
	y0 := y - Y0
	
	// Determine which simplex we're in
	var i1, j1 int
	if x0 > y0 {
		i1, j1 = 1, 0
	} else {
		i1, j1 = 0, 1
	}
	
	// Calculate corners coordinates
	x1 := x0 - float64(i1) + G2
	y1 := y0 - float64(j1) + G2
	x2 := x0 - 1.0 + 2.0*G2
	y2 := y0 - 1.0 + 2.0*G2
	
	// Calculate contribution from each corner
	n0, n1, n2 := 0.0, 0.0, 0.0
	
	// Corner 0
	t0 := 0.5 - x0*x0 - y0*y0
	if t0 > 0 {
		g := gradient2D(hash(int(i), int(j), 0, int(seed)))
		t0 *= t0
		n0 = t0 * t0 * dot2D(g[0], g[1], x0, y0)
	}
	
	// Corner 1
	t1 := 0.5 - x1*x1 - y1*y1
	if t1 > 0 {
		g := gradient2D(hash(int(i)+i1, int(j)+j1, 0, int(seed)))
		t1 *= t1
		n1 = t1 * t1 * dot2D(g[0], g[1], x1, y1)
	}
	
	// Corner 2
	t2 := 0.5 - x2*x2 - y2*y2
	if t2 > 0 {
		g := gradient2D(hash(int(i)+1, int(j)+1, 0, int(seed)))
		t2 *= t2
		n2 = t2 * t2 * dot2D(g[0], g[1], x2, y2)
	}
	
	// Return scaled result (70.0 is a normalization factor)
	return 70.0 * (n0 + n1 + n2)
}

// Helper functions

// hash combines the coordinates and seed to create a unique hash
func hash(x, y, z, seed int) int {
	h := seed + x*374761393 + y*668265263 + z*374761393
	h = (h ^ (h >> 13)) * 1274126177
	return h ^ (h >> 16)
}

// hashToFloat converts a hash to a float in range [0, 1)
func hashToFloat(h int) float64 {
	return float64(h&0xFFFFFF) / 16777216.0
}

// gradient1D generates a 1D gradient from a hash
func gradient1D(hash int) float64 {
	h := hash & 1
	if h == 0 {
		return 1.0
	}
	return -1.0
}

// gradient2D generates a 2D gradient from a hash
func gradient2D(hash int) [2]float64 {
	h := hash & 7
	switch h {
	case 0:
		return [2]float64{1, 0}
	case 1:
		return [2]float64{-1, 0}
	case 2:
		return [2]float64{0, 1}
	case 3:
		return [2]float64{0, -1}
	case 4:
		return [2]float64{1, 1}
	case 5:
		return [2]float64{-1, 1}
	case 6:
		return [2]float64{1, -1}
	default:
		return [2]float64{-1, -1}
	}
}

// gradient3D generates a 3D gradient from a hash
func gradient3D(hash int) [3]float64 {
	h := hash & 15
	
	u := float64(1)
	if h&8 != 0 {
		u = -1
	}
	
	v := float64(1)
	if h&4 != 0 {
		v = -1
	}
	
	// Compute gradient based on hash
	var dx, dy, dz float64
	
	if h&1 != 0 {
		dx = u
	} else {
		dx = 0
	}
	
	if h&2 != 0 {
		dy = v
	} else {
		dy = 0
	}
	
	if dx == 0 && dy == 0 {
		if h&3 == 0 {
			dz = u
		} else {
			dz = -u
		}
	} else {
		dz = 0
	}
	
	// Normalize gradient
	length := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if length > 0 {
		dx /= length
		dy /= length
		dz /= length
	}
	
	return [3]float64{dx, dy, dz}
}

// dot2D calculates 2D dot product
func dot2D(x1, y1, x2, y2 float64) float64 {
	return x1*x2 + y1*y2
}

// dot3D calculates 3D dot product
func dot3D(x1, y1, z1, x2, y2, z2 float64) float64 {
	return x1*x2 + y1*y2 + z1*z2
}

// lerp performs linear interpolation
func lerp(a, b, t float64) float64 {
	return a + t*(b-a)
}

// smoothstep applies a smoothing function to t
func smoothstep(t float64) float64 {
	// Improved Perlin smoothstep: 6t^5 - 15t^4 + 10t^3
	return t * t * t * (t * (t*6.0 - 15.0) + 10.0)
}
}

// NewNoiseGenerator creates a new noise generator with the given seed
func NewNoiseGenerator(seed int64) *NoiseGenerator {
	return &NoiseGenerator{
		rng: rand.New(rand.NewSource(seed)),
	}
}

// RandomFloat returns a random float in range [0.0, 1.0)
func (ng *NoiseGenerator) RandomFloat() float64 {
	return ng.rng.Float64()
}

// RandomRange returns a random float in range [min, max)
func (ng *NoiseGenerator) RandomRange(min, max float64) float64 {
	return min + ng.rng.Float64()*(max-min)
}

// Perlin1D generates 1D Perlin noise
func (ng *NoiseGenerator) Perlin1D(x float64, seed int64) float64 {
	// Get grid points
	x0 := math.Floor(x)
	x1 := x0 + 1.0
	
	// Get interpolation factor
	sx := x - x0
	
	// Smooth interpolation factor
	sx = smoothstep(sx)
	
	// Get gradients
	g0 := gradient1D(hash(int(x0), 0, 0, int(seed)))
	g1 := gradient1D(hash(int(x1), 0, 0, int(seed)))
	
	// Calculate dot products
	v0 := g0 * (x - x0)
	v1 := g1 * (x - x1)
	
	// Interpolate
	return lerp(v0, v1, sx) * 2.0
}

// Perlin2D generates 2D Perlin noise
func (ng *NoiseGenerator) Perlin2D(x, y float64, seed int64) float64 {
	// Get grid points
	x0 := math.Floor(x)
	x1 := x0 + 1.0
	y0 := math.Floor(y)
	y1 := y0 + 1.0
	
	// Get interpolation factors
	sx := x - x0
	sy := y - y0
	
	// Smooth interpolation factors
	sx = smoothstep(sx)
	sy = smoothstep(sy)
	
	// Get gradients
	g00 := gradient2D(hash(int(x0), int(y0), 0, int(seed)))
	g10 := gradient2D(hash(int(x1), int(y0), 0, int(seed)))
	g01 := gradient2D(hash(int(x0), int(y1), 0, int(seed)))
	g11 := gradient2D(hash(int(x1), int(y1), 0, int(seed)))
	
	// Calculate dot products
	dp00 := dot2D(g00[0], g00[1], x-x0, y-y0)
	dp10 := dot2D(g10[0], g10[1], x-x1, y-y0)
	dp01 := dot2D(g01[0], g01[1], x-x0, y-y1)
	dp11 := dot2D(g11[0], g11[1], x-x1, y-y1)
	
	// Interpolate along x
	v0 := lerp(dp00, dp10, sx)
	v1 := lerp(dp01, dp11, sx)
	
	// Interpolate along y
	return lerp(v0, v1, sy)
}

// Perlin3D generates 3D Perlin noise
func (ng *NoiseGenerator) Perlin3D(x, y, z float64, seed int64) float64 {
	// Get grid points
	x0 := math.Floor(x)
	x1 := x0 + 1.0
	y0 := math.Floor(y)
	y1 := y0 + 1.0
	z0 := math.Floor(z)
	z1 := z0 + 1.0
	
	// Get interpolation factors
	sx := x - x0
	sy := y - y0
	sz := z - z0
	
	// Smooth interpolation factors
	sx = smoothstep(sx)
	sy = smoothstep(sy)
	sz = smoothstep(sz)
	
	// Get gradients
	g000 := gradient3D(hash(int(x0), int(y0), int(z0), int(seed)))
	g100 := gradient3D(hash(int(x1), int(y0), int(z0), int(seed)))
	g010 := gradient3D(hash(int(x0), int(y1), int(z0), int(seed)))
	g110 := gradient3D(hash(int(x1), int(y1), int(z0), int(seed)))
	g001 := gradient3D(hash(int(x0), int(y0), int(z1), int(seed)))
	g101 := gradient3D(hash(int(x1), int(y0), int(z1), int(seed)))
	g011 := gradient3D(hash(int(x0), int(y1), int(z1), int(seed)))
	g111 := gradient3D(hash(int(x1), int(y1), int(z1), int(seed)))
	
	// Calculate dot products
	dp000 := dot3D(g000[0], g000[1], g000[2], x-x0, y-y0, z-z0)
	dp100 := dot3D(g100[0], g100[1], g100[2], x-x1, y-y0, z-z0)
	dp010 := dot3D(g010[0], g010[1], g010[2], x-x0, y-y1, z-z0)
	dp110 := dot3D(g110[0], g110[1], g110[2], x-x1, y-y1, z-z0)
	dp001 := dot3D(g001[0], g001[1], g001[2], x-x0, y-y0, z-z1)
	dp101 := dot3D(g101[0], g101[1], g101[2], x-x1, y-y0, z-z1)
	dp011 := dot3D(g011[0], g011[1], g011[2], x-x0, y-y1, z-z1)
	dp111 := dot3D(g111[0], g111[1], g111[2], x-x1, y-y1, z-z1)
	
	// Interpolate along x (front face)
	v00 := lerp(dp000, dp100, sx)
	v10 := lerp(dp010, dp110, sx)
	// Interpolate along x (back face)
	v01 := lerp(dp001, dp101, sx)
	v11 := lerp(dp011, dp111, sx)
	
	// Interpolate along y
	v0 := lerp(v00, v10, sy)
	v1 := lerp(v01, v11, sy)
	
	// Interpolate along z
	return lerp(v0, v1, sz)
}