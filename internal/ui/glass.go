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

// Kage shader for simple dimming (modal backgrounds)
var dimmingShader = []byte(`
//kage:unit pixels

package main

var DimAmount float

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
    bg := imageSrc0At(srcPos)
    // Apply dimming by darkening the blurred background
    return vec4(bg.rgb * (1.0 - DimAmount), 1.0)
}
`)

// Kage shader for liquid glass with SDF, fresnel, chromatic aberration
var liquidGlassShader = []byte(`
//kage:unit pixels

package main

var Time float
var CenterX float
var CenterY float
var Width float
var Height float
var CornerRadius float
var TintR float
var TintG float
var TintB float
var TintA float

// Rounded rectangle SDF
func sdRoundedRect(p vec2, size vec2, r float) float {
    q := abs(p) - size + vec2(r, r)
    return min(max(q.x, q.y), 0.0) + length(max(q, vec2(0.0, 0.0))) - r
}

// Fresnel (Schlick approximation)
func fresnelSchlick(cosTheta float) float {
    r0 := 0.04
    return r0 + (1.0 - r0) * pow(clamp(1.0 - cosTheta, 0.0, 1.0), 5.0)
}

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
    center := vec2(CenterX, CenterY)
    size := vec2(Width, Height) * 0.5
    p := srcPos - center

    // Distance to rounded rect
    d := sdRoundedRect(p, size, CornerRadius)

    // Drop shadow (offset SDF)
    shadowOffset := vec2(0.0, 4.0)
    shadowD := sdRoundedRect(p - shadowOffset, size, CornerRadius)
    shadowMask := 1.0 - smoothstep(-8.0, 0.0, shadowD)
    shadowMask *= 0.15

    // Inside the glass
    if d < 0.0 {
        // Distance from edge (for fresnel)
        minSize := min(size.x, size.y)
        edgeDist := -d / (minSize * 0.5)
        edgeDist = clamp(edgeDist, 0.0, 1.0)

        // Lens distortion (magnify center, distort edges)
        distortStrength := 0.02 * (1.0 - edgeDist)
        offset := p * distortStrength

        // Chromatic aberration
        redUV := srcPos + offset * 0.9
        greenUV := srcPos + offset * 1.0
        blueUV := srcPos + offset * 1.1

        r := imageSrc0At(redUV).r
        g := imageSrc0At(greenUV).g
        b := imageSrc0At(blueUV).b

        refracted := vec3(r, g, b)

        // Brighten slightly
        refracted = refracted * 1.05 + vec3(0.02)

        // Fresnel edge highlight
        fresnelAmount := fresnelSchlick(edgeDist)
        highlight := vec3(1.0, 1.0, 1.0) * fresnelAmount * 0.3

        // Tint
        tint := vec4(TintR, TintG, TintB, TintA)
        finalColor := mix(refracted + highlight, tint.rgb, tint.a)

        // Edge line (subtle white border)
        edgeLine := smoothstep(-2.0, 0.0, d) * smoothstep(0.0, -1.0, d)
        finalColor = mix(finalColor, vec3(1.0), edgeLine * 0.5)

        return vec4(finalColor, 1.0)
    }

    // Outside - show background with shadow
    bg := imageSrc0At(srcPos).rgb
    bg = mix(bg, vec3(0.0), shadowMask)
    return vec4(bg, 1.0)
}
`)

// GlassEffect manages liquid glass blur rendering
type GlassEffect struct {
	blurH   *ebiten.Shader
	blurV   *ebiten.Shader
	glass   *ebiten.Shader
	dimming *ebiten.Shader // For modal backgrounds
	tempH   *ebiten.Image  // Horizontal blur result
	tempV   *ebiten.Image  // Vertical blur result
	time    float64
	enabled bool

	// Cached modal background (captured once when modal opens)
	modalCache *ebiten.Image
	modalW     int
	modalH     int
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

	ge.dimming, err = ebiten.NewShader(dimmingShader)
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

	// Apply liquid glass effect with SDF, fresnel, and chromatic aberration
	// Center coordinates are relative to the shader's local coordinate system
	glassOp := &ebiten.DrawRectShaderOptions{
		Uniforms: map[string]interface{}{
			"Time":         float32(ge.time),
			"CenterX":      float32(w) / 2.0,
			"CenterY":      float32(h) / 2.0,
			"Width":        float32(w),
			"Height":       float32(h),
			"CornerRadius": float32(8.0), // Default corner radius
			"TintR":        float32(tint.R) / 255.0,
			"TintG":        float32(tint.G) / 255.0,
			"TintB":        float32(tint.B) / 255.0,
			"TintA":        float32(tint.A) / 255.0,
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

// CaptureForModal captures and blurs the current screen for modal use.
// Call this ONCE when modal opens, not every frame, to avoid flicker.
func (ge *GlassEffect) CaptureForModal(screen *ebiten.Image, sigma float64) {
	if !ge.IsEnabled() {
		return
	}

	bounds := screen.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	if w <= 0 || h <= 0 {
		return
	}

	// Ensure cache image exists and is correct size
	if ge.modalCache == nil || ge.modalW != w || ge.modalH != h {
		ge.modalCache = ebiten.NewImage(w, h)
		ge.modalW = w
		ge.modalH = h
	}

	// Ensure temp images are correct size
	ge.ensureImages(w, h)

	// Capture screen to tempH
	ge.tempH.Clear()
	op := &ebiten.DrawImageOptions{}
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

	// Apply vertical blur: tempV -> modalCache
	ge.modalCache.Clear()
	blurOpV := &ebiten.DrawRectShaderOptions{
		Uniforms: map[string]interface{}{
			"Sigma": float32(sigma),
		},
		Images: [4]*ebiten.Image{ge.tempV},
	}
	ge.modalCache.DrawRectShader(w, h, ge.blurV, blurOpV)
}

// DrawModalBackground draws the cached blurred background with dimming.
// Use this for modal backgrounds - NOT for material surfaces.
func (ge *GlassEffect) DrawModalBackground(screen *ebiten.Image, dimAmount float64) {
	if !ge.IsEnabled() || ge.modalCache == nil {
		// Fallback: simple dark overlay
		ge.drawModalFallback(screen, dimAmount)
		return
	}

	bounds := screen.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Apply dimming shader to cached blur and draw to screen
	dimOp := &ebiten.DrawRectShaderOptions{
		Uniforms: map[string]interface{}{
			"DimAmount": float32(dimAmount),
		},
		Images: [4]*ebiten.Image{ge.modalCache},
	}
	screen.DrawRectShader(w, h, ge.dimming, dimOp)
}

// drawModalFallback draws a simple semi-transparent dark overlay
func (ge *GlassEffect) drawModalFallback(screen *ebiten.Image, dimAmount float64) {
	bounds := screen.Bounds()
	alpha := uint8(dimAmount * 255)
	overlay := color.RGBA{0, 0, 0, alpha}

	fallbackImg := ebiten.NewImage(bounds.Dx(), bounds.Dy())
	fallbackImg.Fill(overlay)
	screen.DrawImage(fallbackImg, nil)
}

// InvalidateModalCache clears the modal cache (call when modal closes)
func (ge *GlassEffect) InvalidateModalCache() {
	if ge.modalCache != nil {
		ge.modalCache.Clear()
	}
}
