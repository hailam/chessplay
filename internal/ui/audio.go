// Package ui implements the chess game UI using Ebitengine.
package ui

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

// SoundType represents different sound effects.
type SoundType int

const (
	SoundMove SoundType = iota
	SoundCapture
	SoundCheck
	SoundCastle
	SoundInvalid
	SoundGameEnd
)

const (
	sampleRate = 44100
)

// AudioManager handles sound effect playback.
type AudioManager struct {
	context *audio.Context
	sounds  map[SoundType][]byte
	enabled bool
	volume  float64
}

// NewAudioManager creates a new audio manager.
func NewAudioManager() *AudioManager {
	am := &AudioManager{
		context: audio.NewContext(sampleRate),
		sounds:  make(map[SoundType][]byte),
		enabled: true,
		volume:  0.5,
	}
	am.generateSounds()
	return am
}

// generateSounds creates procedural sounds for each event type.
func (am *AudioManager) generateSounds() {
	// Move sound: short click (wood on wood)
	am.sounds[SoundMove] = am.generateClick(440, 0.08, 0.3)

	// Capture sound: sharper impact
	am.sounds[SoundCapture] = am.generateClick(330, 0.12, 0.5)

	// Check sound: alert tone
	am.sounds[SoundCheck] = am.generateTone(880, 0.15, 0.4)

	// Castle sound: double click
	am.sounds[SoundCastle] = am.generateDoubleClick(400, 0.06, 0.3)

	// Invalid sound: low buzz
	am.sounds[SoundInvalid] = am.generateBuzz(150, 0.1, 0.3)

	// Game end sound: chord
	am.sounds[SoundGameEnd] = am.generateChord(0.4, 0.5)
}

// generateClick creates a short percussive click sound.
func (am *AudioManager) generateClick(freq float64, duration float64, amplitude float64) []byte {
	samples := int(sampleRate * duration)
	data := make([]byte, samples*4) // stereo 16-bit

	for i := 0; i < samples; i++ {
		t := float64(i) / sampleRate
		// Exponential decay envelope
		envelope := math.Exp(-t * 30)
		// Add some noise for wood texture
		noise := (math.Sin(float64(i)*0.3) + math.Sin(float64(i)*0.7)) * 0.3
		sample := (math.Sin(2*math.Pi*freq*t) + noise) * envelope * amplitude

		// Clamp and convert to 16-bit signed
		val := int16(sample * 32767)
		// Write stereo samples (left and right)
		data[i*4] = byte(val)
		data[i*4+1] = byte(val >> 8)
		data[i*4+2] = byte(val)
		data[i*4+3] = byte(val >> 8)
	}
	return data
}

// generateTone creates a simple tone with attack and decay.
func (am *AudioManager) generateTone(freq float64, duration float64, amplitude float64) []byte {
	samples := int(sampleRate * duration)
	data := make([]byte, samples*4)

	for i := 0; i < samples; i++ {
		t := float64(i) / sampleRate
		progress := t / duration
		// Attack-decay envelope
		var envelope float64
		if progress < 0.1 {
			envelope = progress / 0.1
		} else {
			envelope = 1.0 - (progress-0.1)/0.9
		}
		sample := math.Sin(2*math.Pi*freq*t) * envelope * amplitude

		val := int16(sample * 32767)
		data[i*4] = byte(val)
		data[i*4+1] = byte(val >> 8)
		data[i*4+2] = byte(val)
		data[i*4+3] = byte(val >> 8)
	}
	return data
}

// generateDoubleClick creates two quick clicks.
func (am *AudioManager) generateDoubleClick(freq float64, duration float64, amplitude float64) []byte {
	click1 := am.generateClick(freq, duration, amplitude)
	silence := make([]byte, int(sampleRate*0.05)*4) // 50ms gap
	click2 := am.generateClick(freq*1.1, duration, amplitude*0.8)

	result := make([]byte, 0, len(click1)+len(silence)+len(click2))
	result = append(result, click1...)
	result = append(result, silence...)
	result = append(result, click2...)
	return result
}

// generateBuzz creates a low error buzz.
func (am *AudioManager) generateBuzz(freq float64, duration float64, amplitude float64) []byte {
	samples := int(sampleRate * duration)
	data := make([]byte, samples*4)

	for i := 0; i < samples; i++ {
		t := float64(i) / sampleRate
		progress := t / duration
		envelope := 1.0 - progress // Linear decay
		// Square-ish wave for buzz effect
		wave := math.Sin(2*math.Pi*freq*t) + 0.3*math.Sin(4*math.Pi*freq*t)
		sample := wave * envelope * amplitude * 0.5

		val := int16(sample * 32767)
		data[i*4] = byte(val)
		data[i*4+1] = byte(val >> 8)
		data[i*4+2] = byte(val)
		data[i*4+3] = byte(val >> 8)
	}
	return data
}

// generateChord creates a simple major chord.
func (am *AudioManager) generateChord(duration float64, amplitude float64) []byte {
	samples := int(sampleRate * duration)
	data := make([]byte, samples*4)

	// C major chord: C4, E4, G4
	freqs := []float64{261.63, 329.63, 392.00}

	for i := 0; i < samples; i++ {
		t := float64(i) / sampleRate
		progress := t / duration
		// Fade in then out
		var envelope float64
		if progress < 0.1 {
			envelope = progress / 0.1
		} else if progress > 0.7 {
			envelope = (1.0 - progress) / 0.3
		} else {
			envelope = 1.0
		}

		sample := 0.0
		for _, freq := range freqs {
			sample += math.Sin(2 * math.Pi * freq * t)
		}
		sample = sample / float64(len(freqs)) * envelope * amplitude

		val := int16(sample * 32767)
		data[i*4] = byte(val)
		data[i*4+1] = byte(val >> 8)
		data[i*4+2] = byte(val)
		data[i*4+3] = byte(val >> 8)
	}
	return data
}

// Play plays a sound effect.
func (am *AudioManager) Play(sound SoundType) {
	if !am.enabled {
		return
	}

	data, ok := am.sounds[sound]
	if !ok {
		return
	}

	// Create a new player for each play (allows overlapping sounds)
	player := am.context.NewPlayerFromBytes(data)
	player.SetVolume(am.volume)
	player.Play()
}

// SetEnabled enables or disables audio.
func (am *AudioManager) SetEnabled(enabled bool) {
	am.enabled = enabled
}

// SetVolume sets the audio volume (0.0 to 1.0).
func (am *AudioManager) SetVolume(volume float64) {
	if volume < 0 {
		volume = 0
	}
	if volume > 1 {
		volume = 1
	}
	am.volume = volume
}

// IsEnabled returns whether audio is enabled.
func (am *AudioManager) IsEnabled() bool {
	return am.enabled
}
