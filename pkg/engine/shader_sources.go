package engine

// Shader sources for the OpenGL renderer

// Basic vertex shader for rendering ASCII characters
const vertexShaderSource = `
#version 410 core
layout (location = 0) in vec3 aPos;
layout (location = 1) in vec2 aTexCoord;

out vec2 TexCoord;

void main() {
    gl_Position = vec4(aPos, 1.0);
    TexCoord = aTexCoord;
}
`

// Fragment shader for rendering ASCII characters
const fragmentShaderSource = `
#version 410 core
in vec2 TexCoord;
out vec4 FragColor;

uniform sampler2D asciiTexture;
uniform vec3 textColor;

void main() {
    // Sample the texture
    float alpha = texture(asciiTexture, TexCoord).r;
    
    // Apply text color
    FragColor = vec4(textColor * alpha, alpha);
}
`

// Vertex shader for post-processing effects
const postProcessVertexShader = `
#version 410 core
layout (location = 0) in vec3 aPos;
layout (location = 1) in vec2 aTexCoord;

out vec2 TexCoord;

void main() {
    gl_Position = vec4(aPos, 1.0);
    TexCoord = aTexCoord;
}
`

// Fragment shader for post-processing effects
const postProcessFragmentShader = `
#version 410 core
in vec2 TexCoord;
out vec4 FragColor;

uniform sampler2D screenTexture;
uniform float time;
uniform float glitchAmount;
uniform float vignetteAmount;
uniform float noiseAmount;
uniform vec2 resolution;

// Pseudo-random noise function
float rand(vec2 co) {
    return fract(sin(dot(co.xy, vec2(12.9898, 78.233))) * 43758.5453);
}

// Simple noise function
float noise(vec2 p) {
    vec2 ip = floor(p);
    vec2 u = fract(p);
    u = u * u * (3.0 - 2.0 * u);
    
    float res = mix(
        mix(rand(ip), rand(ip + vec2(1.0, 0.0)), u.x),
        mix(rand(ip + vec2(0.0, 1.0)), rand(ip + vec2(1.0, 1.0)), u.x),
        u.y);
    return res * res;
}

void main() {
    // Base texture coordinates
    vec2 texCoord = TexCoord;
    
    // Apply glitch effect
    if (glitchAmount > 0.0) {
        // Random horizontal glitch lines
        float lineNoise = floor(texCoord.y * 20.0) / 20.0 * time;
        
        if (rand(vec2(lineNoise, time * 0.1)) < glitchAmount * 0.8) {
            // Apply random RGB shift to create color fringing
            float shift = (rand(vec2(lineNoise, time * 0.1)) - 0.5) * 0.01 * glitchAmount;
            
            float redShift = shift * 1.5;
            float greenShift = shift * -0.5;
            float blueShift = shift * -1.0;
            
            vec4 rSample = texture(screenTexture, vec2(texCoord.x + redShift, texCoord.y));
            vec4 gSample = texture(screenTexture, vec2(texCoord.x + greenShift, texCoord.y));
            vec4 bSample = texture(screenTexture, vec2(texCoord.x + blueShift, texCoord.y));
            
            FragColor = vec4(rSample.r, gSample.g, bSample.b, 1.0);
            return;
        }
        
        // Block displacement glitch
        if (rand(vec2(floor(time * 10.0), floor(time * 20.0))) < glitchAmount * 0.4) {
            float blockSize = rand(vec2(floor(time * 10.0), floor(time * 20.0))) * 0.02 * glitchAmount;
            float blockShift = (rand(vec2(floor(texCoord.y / blockSize), time)) - 0.5) * 0.01 * glitchAmount;
            texCoord.x += blockShift;
        }
    }
    
    // Sample the screen texture
    vec4 color = texture(screenTexture, texCoord);
    
    // Apply vignette effect
    float distanceFromCenter = length(texCoord - 0.5) * 2.0;
    float vignette = 1.0 - distanceFromCenter * vignetteAmount;
    color.rgb *= vignette;
    
    // Apply noise effect
    if (noiseAmount > 0.0) {
        float n = noise(texCoord * 10.0 + time) * noiseAmount;
        color.rgb += (n - noiseAmount * 0.5) * 0.5;
    }
    
    // Apply subtle scanlines
    float scanline = sin(texCoord.y * resolution.y * 0.5) * 0.02 + 1.0;
    color.rgb *= scanline;
    
    // Apply subtle RGB aberration at screen edges
    float aberrationAmount = 0.01 * distanceFromCenter;
    vec4 rSample = texture(screenTexture, vec2(texCoord.x + aberrationAmount, texCoord.y));
    vec4 bSample = texture(screenTexture, vec2(texCoord.x - aberrationAmount, texCoord.y));
    color.r = mix(color.r, rSample.r, 0.3);
    color.b = mix(color.b, bSample.b, 0.3);
    
    // Apply subtle film grain
    float grain = rand(texCoord * time) * 0.03;
    color.rgb += grain - 0.015;
    
    // Final output
    FragColor = color;
}
`
