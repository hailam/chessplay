// Package ui implements the chess game UI using Ebitengine.
package ui

import (
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/hailam/chessplay/internal/board"
)

// InvalidMoveReason represents why a move was rejected.
type InvalidMoveReason int

const (
	ReasonUnknown InvalidMoveReason = iota
	ReasonWouldLeaveKingInCheck
	ReasonBlockedByOwnPiece
	ReasonInvalidPieceMovement
	ReasonNotYourTurn
)

// ToastType represents the type of toast notification.
type ToastType int

const (
	ToastInfo ToastType = iota
	ToastWarning
	ToastError
	ToastSuccess
)

// Toast represents a notification message.
type Toast struct {
	Message   string
	Type      ToastType
	StartTime time.Time
	Duration  time.Duration
}

// ToastManager manages toast notifications.
type ToastManager struct {
	toasts   []*Toast
	maxStack int
}

// NewToastManager creates a new toast manager.
func NewToastManager() *ToastManager {
	return &ToastManager{
		toasts:   make([]*Toast, 0),
		maxStack: 3,
	}
}

// Show displays a new toast notification.
func (tm *ToastManager) Show(message string, toastType ToastType, duration time.Duration) {
	toast := &Toast{
		Message:   message,
		Type:      toastType,
		StartTime: time.Now(),
		Duration:  duration,
	}
	tm.toasts = append(tm.toasts, toast)
	if len(tm.toasts) > tm.maxStack {
		tm.toasts = tm.toasts[1:]
	}
}

// Update removes expired toasts.
func (tm *ToastManager) Update() {
	now := time.Now()
	active := make([]*Toast, 0)
	for _, t := range tm.toasts {
		if now.Sub(t.StartTime) < t.Duration {
			active = append(active, t)
		}
	}
	tm.toasts = active
}

// Draw renders all active toasts.
func (tm *ToastManager) Draw(screen *ebiten.Image) {
	face := GetRegularFace()
	if face == nil {
		return
	}

	y := 50.0
	for _, t := range tm.toasts {
		elapsed := time.Since(t.StartTime).Seconds()
		duration := t.Duration.Seconds()

		// Fade in/out
		alpha := 1.0
		fadeTime := 0.2
		if elapsed < fadeTime {
			alpha = elapsed / fadeTime
		} else if elapsed > duration-fadeTime {
			alpha = (duration - elapsed) / fadeTime
		}

		// Get colors based on type
		var bgColor, textColor color.RGBA
		switch t.Type {
		case ToastWarning:
			bgColor = color.RGBA{180, 140, 20, uint8(220 * alpha)}
			textColor = color.RGBA{40, 30, 0, uint8(255 * alpha)}
		case ToastError:
			bgColor = color.RGBA{180, 50, 50, uint8(220 * alpha)}
			textColor = color.RGBA{255, 255, 255, uint8(255 * alpha)}
		case ToastSuccess:
			bgColor = color.RGBA{50, 150, 50, uint8(220 * alpha)}
			textColor = color.RGBA{255, 255, 255, uint8(255 * alpha)}
		default: // ToastInfo
			bgColor = color.RGBA{50, 100, 150, uint8(220 * alpha)}
			textColor = color.RGBA{255, 255, 255, uint8(255 * alpha)}
		}

		// Measure text
		w, h := MeasureText(t.Message, face)
		padding := 12.0
		boxW := w + padding*2
		boxH := h + padding*2

		// Center horizontally on the board
		x := float64(BoardSize)/2 - boxW/2

		// Draw background
		vector.DrawFilledRect(screen, float32(x), float32(y), float32(boxW), float32(boxH), bgColor, false)

		// Draw text
		op := &text.DrawOptions{}
		op.GeoM.Translate(x+padding, y+padding)
		op.ColorScale.ScaleWithColor(textColor)
		text.Draw(screen, t.Message, face, op)

		y += boxH + 8
	}
}

// ShakeAnimation represents a piece shake effect.
type ShakeAnimation struct {
	Square    board.Square
	StartTime time.Time
	Duration  time.Duration
	Intensity float64
}

// FlashAnimation represents a square flash effect.
type FlashAnimation struct {
	Square    board.Square
	StartTime time.Time
	Duration  time.Duration
	Color     color.RGBA
}

// AnimationManager manages visual animations.
type AnimationManager struct {
	shakes  []*ShakeAnimation
	flashes []*FlashAnimation
}

// NewAnimationManager creates a new animation manager.
func NewAnimationManager() *AnimationManager {
	return &AnimationManager{
		shakes:  make([]*ShakeAnimation, 0),
		flashes: make([]*FlashAnimation, 0),
	}
}

// StartShake begins a shake animation on a square.
func (am *AnimationManager) StartShake(sq board.Square) {
	am.shakes = append(am.shakes, &ShakeAnimation{
		Square:    sq,
		StartTime: time.Now(),
		Duration:  300 * time.Millisecond,
		Intensity: 8.0,
	})
}

// StartFlash begins a flash animation on a square.
func (am *AnimationManager) StartFlash(sq board.Square, c color.RGBA) {
	am.flashes = append(am.flashes, &FlashAnimation{
		Square:    sq,
		StartTime: time.Now(),
		Duration:  400 * time.Millisecond,
		Color:     c,
	})
}

// Update removes expired animations.
func (am *AnimationManager) Update() {
	now := time.Now()

	activeShakes := make([]*ShakeAnimation, 0)
	for _, s := range am.shakes {
		if now.Sub(s.StartTime) < s.Duration {
			activeShakes = append(activeShakes, s)
		}
	}
	am.shakes = activeShakes

	activeFlashes := make([]*FlashAnimation, 0)
	for _, f := range am.flashes {
		if now.Sub(f.StartTime) < f.Duration {
			activeFlashes = append(activeFlashes, f)
		}
	}
	am.flashes = activeFlashes
}

// GetShakeOffset returns the current shake offset for a square.
func (am *AnimationManager) GetShakeOffset(sq board.Square) (float64, float64) {
	for _, s := range am.shakes {
		if s.Square == sq {
			elapsed := time.Since(s.StartTime).Seconds()
			progress := elapsed / s.Duration.Seconds()
			if progress >= 1.0 {
				return 0, 0
			}
			// Damped sine wave oscillation
			decay := 5.0
			freq := 40.0
			amplitude := s.Intensity * math.Exp(-decay*progress)
			offset := amplitude * math.Sin(freq*progress)
			return offset, 0
		}
	}
	return 0, 0
}

// GetFlashForSquare returns the active flash for a square, if any.
func (am *AnimationManager) GetFlashForSquare(sq board.Square) *FlashAnimation {
	for _, f := range am.flashes {
		if f.Square == sq {
			return f
		}
	}
	return nil
}

// DrawFlashes renders all active flash overlays.
func (am *AnimationManager) DrawFlashes(screen *ebiten.Image, renderer *Renderer) {
	for _, f := range am.flashes {
		elapsed := time.Since(f.StartTime).Seconds()
		progress := elapsed / f.Duration.Seconds()
		if progress >= 1.0 {
			continue
		}

		// Fade out
		alpha := 1.0 - progress
		c := color.RGBA{f.Color.R, f.Color.G, f.Color.B, uint8(float64(f.Color.A) * alpha)}

		x, y := renderer.SquareToScreen(f.Square)
		size := float32(renderer.SquareSize())
		vector.DrawFilledRect(screen, float32(x), float32(y), size, size, c, false)
	}
}

// FeedbackManager coordinates all feedback systems.
type FeedbackManager struct {
	toasts     *ToastManager
	animations *AnimationManager
	audio      *AudioManager
}

// NewFeedbackManager creates a new feedback manager.
func NewFeedbackManager() *FeedbackManager {
	return &FeedbackManager{
		toasts:     NewToastManager(),
		animations: NewAnimationManager(),
		audio:      NewAudioManager(),
	}
}

// Update updates all feedback systems.
func (fm *FeedbackManager) Update() {
	fm.toasts.Update()
	fm.animations.Update()
}

// Draw renders all feedback overlays.
func (fm *FeedbackManager) Draw(screen *ebiten.Image, renderer *Renderer) {
	fm.animations.DrawFlashes(screen, renderer)
	fm.toasts.Draw(screen)
}

// Animations returns the animation manager for renderer integration.
func (fm *FeedbackManager) Animations() *AnimationManager {
	return fm.animations
}

// OnInvalidMove handles an invalid move attempt.
func (fm *FeedbackManager) OnInvalidMove(from, to board.Square, reason InvalidMoveReason) {
	var message string
	switch reason {
	case ReasonWouldLeaveKingInCheck:
		message = "Illegal move - King would be in check"
	case ReasonBlockedByOwnPiece:
		message = "Square occupied by your piece"
	case ReasonInvalidPieceMovement:
		message = "Invalid move for this piece"
	case ReasonNotYourTurn:
		message = "Not your turn"
	default:
		message = "Invalid move"
	}

	fm.toasts.Show(message, ToastWarning, 2*time.Second)
	fm.animations.StartShake(from)
	fm.animations.StartFlash(to, color.RGBA{255, 80, 80, 150})
	fm.audio.Play(SoundInvalid)
}

// OnCheck handles a check event.
func (fm *FeedbackManager) OnCheck() {
	fm.toasts.Show("Check!", ToastWarning, 2*time.Second)
	fm.audio.Play(SoundCheck)
}

// OnCheckmate handles a checkmate event.
func (fm *FeedbackManager) OnCheckmate(winner board.Color) {
	var message string
	if winner == board.White {
		message = "Checkmate! White wins!"
	} else {
		message = "Checkmate! Black wins!"
	}
	fm.toasts.Show(message, ToastSuccess, 5*time.Second)
	fm.audio.Play(SoundGameEnd)
}

// OnStalemate handles a stalemate event.
func (fm *FeedbackManager) OnStalemate() {
	fm.toasts.Show("Stalemate - Draw", ToastInfo, 5*time.Second)
	fm.audio.Play(SoundGameEnd)
}

// OnDraw handles a draw event.
func (fm *FeedbackManager) OnDraw(reason string) {
	fm.toasts.Show("Draw - "+reason, ToastInfo, 5*time.Second)
	fm.audio.Play(SoundGameEnd)
}

// OnMoveMade handles a successful move.
func (fm *FeedbackManager) OnMoveMade(isCapture, isCastling bool) {
	if isCastling {
		fm.audio.Play(SoundCastle)
	} else if isCapture {
		fm.audio.Play(SoundCapture)
	} else {
		fm.audio.Play(SoundMove)
	}
}

// Audio returns the audio manager for settings access.
func (fm *FeedbackManager) Audio() *AudioManager {
	return fm.audio
}
