package ui

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// Kage shader for Gaussian blur (horizontal pass)
// Uses 9-tap Gaussian kernel (fixed size for Kage compatibility)
var blurHorizontalShader = []byte(`
//kage:unit pixels

package main

var Sigma float  // Controls blur strength (pixel spread)

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
    // 9-tap Gaussian blur weights (precomputed, sums to 1.0)
    var result vec4

    result += imageSrc0At(srcPos + vec2(-4*Sigma, 0)) * 0.0162
    result += imageSrc0At(srcPos + vec2(-3*Sigma, 0)) * 0.0540
    result += imageSrc0At(srcPos + vec2(-2*Sigma, 0)) * 0.1218
    result += imageSrc0At(srcPos + vec2(-1*Sigma, 0)) * 0.1954
    result += imageSrc0At(srcPos + vec2(0, 0)) * 0.2252
    result += imageSrc0At(srcPos + vec2(1*Sigma, 0)) * 0.1954
    result += imageSrc0At(srcPos + vec2(2*Sigma, 0)) * 0.1218
    result += imageSrc0At(srcPos + vec2(3*Sigma, 0)) * 0.0540
    result += imageSrc0At(srcPos + vec2(4*Sigma, 0)) * 0.0162

    return result
}
`)

// Kage shader for Gaussian blur (vertical pass)
var blurVerticalShader = []byte(`
//kage:unit pixels

package main

var Sigma float

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
    var result vec4

    result += imageSrc0At(srcPos + vec2(0, -4*Sigma)) * 0.0162
    result += imageSrc0At(srcPos + vec2(0, -3*Sigma)) * 0.0540
    result += imageSrc0At(srcPos + vec2(0, -2*Sigma)) * 0.1218
    result += imageSrc0At(srcPos + vec2(0, -1*Sigma)) * 0.1954
    result += imageSrc0At(srcPos + vec2(0, 0)) * 0.2252
    result += imageSrc0At(srcPos + vec2(0, 1*Sigma)) * 0.1954
    result += imageSrc0At(srcPos + vec2(0, 2*Sigma)) * 0.1218
    result += imageSrc0At(srcPos + vec2(0, 3*Sigma)) * 0.0540
    result += imageSrc0At(srcPos + vec2(0, 4*Sigma)) * 0.0162

    return result
}
`)

// Kage shader for liquid glass refraction + tint
var liquidGlassShader = []byte(`
//kage:unit pixels

package main

var Time float
var TintR float
var TintG float
var TintB float
var TintA float
var RefractionStrength float

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
    // Subtle wave distortion for liquid glass effect
    distortion := vec2(
        sin(srcPos.y * 0.03 + Time * 1.5) * RefractionStrength,
        cos(srcPos.x * 0.03 + Time * 1.2) * RefractionStrength * 0.7,
    )

    // Sample blurred background with refraction offset
    blurred := imageSrc0At(srcPos + distortion)

    // Apply tint overlay
    tint := vec4(TintR, TintG, TintB, TintA)

    // Mix blurred background with tint color based on tint alpha
    return mix(blurred, vec4(tint.rgb, 1.0), tint.a)
}
`)

// GlassEffect manages liquid glass blur rendering
type GlassEffect struct {
	blurH   *ebiten.Shader
	blurV   *ebiten.Shader
	glass   *ebiten.Shader
	tempH   *ebiten.Image // Horizontal blur result
	tempV   *ebiten.Image // Vertical blur result
	time    float64
	enabled bool
}

// NewGlassEffect creates a new glass effect manager
func NewGlassEffect() *GlassEffect {
	ge := &GlassEffect{
		enabled: true,
	}

	var err error
	ge.blurH, err = ebiten.NewShader(blurHorizontalShader)
	if err != nil {
		ge.enabled = false
		return ge
	}

	ge.blurV, err = ebiten.NewShader(blurVerticalShader)
	if err != nil {
		ge.enabled = false
		return ge
	}

	ge.glass, err = ebiten.NewShader(liquidGlassShader)
	if err != nil {
		ge.enabled = false
		return ge
	}

	return ge
}

// IsEnabled returns whether the glass effect is available
func (ge *GlassEffect) IsEnabled() bool {
	return ge != nil && ge.enabled
}

// Update updates the time for animation
func (ge *GlassEffect) Update() {
	if ge == nil {
		return
	}
	ge.time += 1.0 / 60.0 // Assuming 60 FPS
}

// ensureImages creates or resizes offscreen images as needed
func (ge *GlassEffect) ensureImages(w, h int) {
	if ge.tempH == nil || ge.tempH.Bounds().Dx() != w || ge.tempH.Bounds().Dy() != h {
		ge.tempH = ebiten.NewImage(w, h)
	}
	if ge.tempV == nil || ge.tempV.Bounds().Dx() != w || ge.tempV.Bounds().Dy() != h {
		ge.tempV = ebiten.NewImage(w, h)
	}
}

// DrawGlass renders a liquid glass effect in the specified region
// x, y, w, h are in screen coordinates (already scaled)
// sigma controls blur strength (1.0-4.0 recommended)
// refractionStrength controls distortion (2.0-8.0 recommended)
func (ge *GlassEffect) DrawGlass(screen *ebiten.Image, x, y, w, h int, tint color.RGBA, sigma, refractionStrength float64) {
	if !ge.IsEnabled() {
		// Fallback: draw semi-transparent overlay
		ge.drawFallback(screen, x, y, w, h, tint)
		return
	}

	if w <= 0 || h <= 0 {
		return
	}

	// Ensure temp images are correct size
	ge.ensureImages(w, h)

	// Clear temp images
	ge.tempH.Clear()
	ge.tempV.Clear()

	// Capture the region from screen to tempH
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(-x), float64(-y))
	ge.tempH.DrawImage(screen, op)

	// Apply horizontal blur: tempH -> tempV
	ge.tempV.Clear()
	blurOpH := &ebiten.DrawRectShaderOptions{
		Uniforms: map[string]interface{}{
			"Sigma": float32(sigma),
		},
		Images: [4]*ebiten.Image{ge.tempH},
	}
	ge.tempV.DrawRectShader(w, h, ge.blurH, blurOpH)

	// Apply vertical blur: tempV -> tempH
	ge.tempH.Clear()
	blurOpV := &ebiten.DrawRectShaderOptions{
		Uniforms: map[string]interface{}{
			"Sigma": float32(sigma),
		},
		Images: [4]*ebiten.Image{ge.tempV},
	}
	ge.tempH.DrawRectShader(w, h, ge.blurV, blurOpV)

	// Apply liquid glass effect with refraction and tint, draw back to screen
	glassOp := &ebiten.DrawRectShaderOptions{
		Uniforms: map[string]interface{}{
			"Time":               float32(ge.time),
			"TintR":              float32(tint.R) / 255.0,
			"TintG":              float32(tint.G) / 255.0,
			"TintB":              float32(tint.B) / 255.0,
			"TintA":              float32(tint.A) / 255.0,
			"RefractionStrength": float32(refractionStrength),
		},
		Images: [4]*ebiten.Image{ge.tempH},
		GeoM:   ebiten.GeoM{},
	}
	glassOp.GeoM.Translate(float64(x), float64(y))
	screen.DrawRectShader(w, h, ge.glass, glassOp)
}

// drawFallback draws a simple semi-transparent overlay when shaders are unavailable
func (ge *GlassEffect) drawFallback(screen *ebiten.Image, x, y, w, h int, tint color.RGBA) {
	fallbackImg := ebiten.NewImage(w, h)
	fallbackImg.Fill(tint)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	screen.DrawImage(fallbackImg, op)
}

// DrawGlassSimple is a convenience method with default refraction strength
func (ge *GlassEffect) DrawGlassSimple(screen *ebiten.Image, x, y, w, h int, tint color.RGBA, sigma float64) {
	ge.DrawGlass(screen, x, y, w, h, tint, sigma, 3.0)
}

// DrawGlassRect draws glass effect using a rectangle (for convenience with image.Rectangle)
func (ge *GlassEffect) DrawGlassRect(screen *ebiten.Image, rect image.Rectangle, tint color.RGBA, sigma float64) {
	ge.DrawGlassSimple(screen, rect.Min.X, rect.Min.Y, rect.Dx(), rect.Dy(), tint, sigma)
}
