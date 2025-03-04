// Add to pkg/engine/physics.go (new file)

package engine

import (
	"math"
	"time"
)

// PhysicsSystem manages physics simulation and collision detection
type PhysicsSystem struct {
	scene        *ProceduralScene
	player       *Player
	gravity      float64
	groundOffset float64 // Height offset from ground
	lastUpdate   time.Time
}

// Player represents the player's physical presence in the world
type Player struct {
	Position       Vector3
	Velocity       Vector3
	Direction      Vector3 // Look direction
	Height         float64
	Radius         float64
	IsGrounded     bool
	IsJumping      bool
	JumpForce      float64
	MoveSpeed      float64
	RotationSpeed  float64
	SprintModifier float64
	StepHeight     float64 // Max height player can step up without jumping
}

// NewPhysicsSystem creates a new physics system
func NewPhysicsSystem() *PhysicsSystem {
	return &PhysicsSystem{
		gravity:      9.8,
		groundOffset: 1.7, // Eye height from ground
		lastUpdate:   time.Now(),
		player: &Player{
			Position:       Vector3{X: 0, Y: 5, Z: 0}, // Start slightly above ground
			Velocity:       Vector3{X: 0, Y: 0, Z: 0},
			Direction:      Vector3{X: 0, Y: 0, Z: 1}, // Looking forward
			Height:         1.8,                       // Player height in meters
			Radius:         0.4,                       // Player collision radius
			IsGrounded:     false,
			IsJumping:      false,
			JumpForce:      5.0,
			MoveSpeed:      5.0, // Units per second
			RotationSpeed:  2.0, // Radians per second
			SprintModifier: 1.8, // Speed multiplier when sprinting
			StepHeight:     0.5, // Can step up half meter obstacles
		},
	}
}

// SetScene sets the current scene for physics calculations
func (ps *PhysicsSystem) SetScene(scene *ProceduralScene) {
	ps.scene = scene
}

// GetPlayer returns the player object
func (ps *PhysicsSystem) GetPlayer() *Player {
	return ps.player
}

// Update updates the physics simulation
func (ps *PhysicsSystem) Update(deltaTime float64) {
	// Skip if we don't have a scene yet
	if ps.scene == nil || ps.scene.Terrain == nil {
		return
	}

	// Apply gravity if not grounded
	if !ps.player.IsGrounded {
		ps.player.Velocity.Y -= ps.gravity * deltaTime
	}

	// Calculate next position based on velocity
	nextPosition := ps.player.Position.Add(ps.player.Velocity.Mul(deltaTime))

	// Check terrain height at current and next position
	currentTerrainHeight := ps.getTerrainHeightAtPosition(ps.player.Position.X, ps.player.Position.Z)
	nextTerrainHeight := ps.getTerrainHeightAtPosition(nextPosition.X, nextPosition.Z)

	// Adjust next position if slope is too steep
	if math.Abs(nextTerrainHeight-currentTerrainHeight) > ps.player.StepHeight && ps.player.IsGrounded {
		slopeRatio := (nextTerrainHeight - currentTerrainHeight) /
			math.Sqrt(math.Pow(nextPosition.X-ps.player.Position.X, 2)+
				math.Pow(nextPosition.Z-ps.player.Position.Z, 2))

		// If slope is too steep (> ~30 degrees), limit horizontal movement
		if math.Abs(slopeRatio) > 0.58 {
			nextPosition.X = ps.player.Position.X
			nextPosition.Z = ps.player.Position.Z
			// Also stop horizontal velocity
			ps.player.Velocity.X = 0
			ps.player.Velocity.Z = 0
		}
	}

	// Check for object collisions
	objectCollision := ps.checkObjectCollisions(nextPosition)
	if objectCollision {
		// Stop movement in case of collision
		nextPosition = ps.player.Position
		ps.player.Velocity.X = 0
		ps.player.Velocity.Z = 0
	}

	// Update player position
	ps.player.Position = nextPosition

	// Check if player is on ground
	groundHeight := ps.getTerrainHeightAtPosition(ps.player.Position.X, ps.player.Position.Z)
	ps.player.IsGrounded = ps.player.Position.Y <= groundHeight+ps.player.Height/2

	// Snap to ground if grounded
	if ps.player.IsGrounded {
		ps.player.Position.Y = groundHeight + ps.player.Height/2
		ps.player.Velocity.Y = 0
		ps.player.IsJumping = false
	}

	// Apply friction if on ground
	if ps.player.IsGrounded {
		ps.player.Velocity.X *= 0.8
		ps.player.Velocity.Z *= 0.8

		// Stop completely if velocity is very small
		if math.Abs(ps.player.Velocity.X) < 0.01 {
			ps.player.Velocity.X = 0
		}
		if math.Abs(ps.player.Velocity.Z) < 0.01 {
			ps.player.Velocity.Z = 0
		}
	}

	// Cap falling velocity to prevent tunneling through terrain
	if ps.player.Velocity.Y < -20 {
		ps.player.Velocity.Y = -20
	}
}

// getTerrainHeightAtPosition gets the terrain height at the given position
func (ps *PhysicsSystem) getTerrainHeightAtPosition(x, z float64) float64 {
	if ps.scene == nil || ps.scene.Terrain == nil {
		return 0
	}

	// Convert world position to terrain grid coordinates
	terrainWidth := ps.scene.Terrain.Width
	terrainHeight := ps.scene.Terrain.Height

	gridX := int(x + float64(terrainWidth)/2)
	gridZ := int(z + float64(terrainHeight)/2)

	// Check bounds
	if gridX < 0 || gridX >= terrainWidth || gridZ < 0 || gridZ >= terrainHeight {
		// Return a very low height for out of bounds
		return -1000
	}

	// Get elevation and scale it
	elevation := ps.scene.Terrain.Data[gridZ][gridX]
	return elevation * 20 // Same scale as used in object placement
}

// Jump makes the player jump if they're on the ground
func (ps *PhysicsSystem) Jump() {
	if ps.player.IsGrounded && !ps.player.IsJumping {
		ps.player.Velocity.Y = ps.player.JumpForce
		ps.player.IsGrounded = false
		ps.player.IsJumping = true
	}
}

// MoveForward moves the player in the direction they're facing
func (ps *PhysicsSystem) MoveForward(sprint bool) {
	speed := ps.player.MoveSpeed
	if sprint {
		speed *= ps.player.SprintModifier
	}

	// Only apply horizontal movement
	moveDir := Vector3{
		X: ps.player.Direction.X,
		Y: 0,
		Z: ps.player.Direction.Z,
	}.Normalize()

	ps.player.Velocity.X = moveDir.X * speed
	ps.player.Velocity.Z = moveDir.Z * speed
}

// MoveBackward moves the player opposite to the direction they're facing
func (ps *PhysicsSystem) MoveBackward() {
	// Only apply horizontal movement
	moveDir := Vector3{
		X: -ps.player.Direction.X,
		Y: 0,
		Z: -ps.player.Direction.Z,
	}.Normalize()

	ps.player.Velocity.X = moveDir.X * ps.player.MoveSpeed * 0.7 // Slower backward movement
	ps.player.Velocity.Z = moveDir.Z * ps.player.MoveSpeed * 0.7
}

// MoveLeft strafes the player to the left
func (ps *PhysicsSystem) MoveLeft() {
	// Calculate left direction (perpendicular to look direction)
	left := Vector3{
		X: -ps.player.Direction.Z,
		Y: 0,
		Z: ps.player.Direction.X,
	}.Normalize()

	ps.player.Velocity.X = left.X * ps.player.MoveSpeed
	ps.player.Velocity.Z = left.Z * ps.player.MoveSpeed
}

// MoveRight strafes the player to the right
func (ps *PhysicsSystem) MoveRight() {
	// Calculate right direction (perpendicular to look direction)
	right := Vector3{
		X: ps.player.Direction.Z,
		Y: 0,
		Z: -ps.player.Direction.X,
	}.Normalize()

	ps.player.Velocity.X = right.X * ps.player.MoveSpeed
	ps.player.Velocity.Z = right.Z * ps.player.MoveSpeed
}

// RotateLeft rotates the player's view to the left
func (ps *PhysicsSystem) RotateLeft(deltaTime float64) {
	// Rotate around Y axis
	angle := ps.player.RotationSpeed * deltaTime

	// Apply rotation to direction vector
	ps.player.Direction = Vector3{
		X: ps.player.Direction.X*math.Cos(angle) - ps.player.Direction.Z*math.Sin(angle),
		Y: ps.player.Direction.Y,
		Z: ps.player.Direction.X*math.Sin(angle) + ps.player.Direction.Z*math.Cos(angle),
	}.Normalize()
}

// RotateRight rotates the player's view to the right
func (ps *PhysicsSystem) RotateRight(deltaTime float64) {
	// Rotate around Y axis
	angle := -ps.player.RotationSpeed * deltaTime

	// Apply rotation to direction vector
	ps.player.Direction = Vector3{
		X: ps.player.Direction.X*math.Cos(angle) - ps.player.Direction.Z*math.Sin(angle),
		Y: ps.player.Direction.Y,
		Z: ps.player.Direction.X*math.Sin(angle) + ps.player.Direction.Z*math.Cos(angle),
	}.Normalize()
}

// checkObjectCollisions checks for collisions with objects in the scene
func (ps *PhysicsSystem) checkObjectCollisions(position Vector3) bool {
	if ps.scene == nil {
		return false
	}

	// Check collision with each object
	for _, obj := range ps.scene.Objects {
		// Simple sphere-sphere collision check
		objPos := obj.Position

		// Calculate collision radius based on object type
		collisionRadius := 1.0 // Default
		switch obj.Type {
		case "tree":
			collisionRadius = obj.Scale.X * 0.8 // Tree trunk
		case "rock":
			collisionRadius = (obj.Scale.X + obj.Scale.Z) / 2 * 0.9
		case "standing_stone":
			collisionRadius = (obj.Scale.X + obj.Scale.Z) / 2 * 0.9
		case "ruin":
			collisionRadius = math.Max(obj.Scale.X, obj.Scale.Z) * 0.8
		case "strange", "altar", "ritual_stone":
			collisionRadius = math.Max(obj.Scale.X, obj.Scale.Z) * 0.9
		}

		// Calculate distance (only in XZ plane)
		dx := position.X - objPos.X
		dz := position.Z - objPos.Z
		distSquared := dx*dx + dz*dz

		minDist := ps.player.Radius + collisionRadius

		if distSquared < minDist*minDist {
			return true // Collision detected
		}
	}

	return false
}
