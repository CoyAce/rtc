package audio

import (
	"math"
	"sync"
)

// ProcessingConfig holds configuration for audio processing
type ProcessingConfig struct {
	// Voice Activity Detection
	EnableVAD    bool    // Whether to enable voice activity detection
	VADThreshold float64 // Energy threshold for voice activity detection (0.0-1.0)
	VADHoldTime  int     // How long to hold voice detection in frames after dropping below threshold

	// Noise Reduction
	EnableNoiseReduction bool    // Whether to enable noise reduction
	NoiseFloor           float32 // Noise floor level (0.0-1.0)
	NoiseAttenuationDB   float32 // Noise attenuation in dB

	// Multi-channel Support
	ChannelCount int  // Number of audio channels
	MixChannels  bool // Whether to mix multiple channels into a single output

	// General Processing
	SampleRate int // Audio sample rate (8000, 16000, 44100, etc.)
	FrameSize  int // Frame size in samples
	BufferSize int // Buffer size for processing
}

// NoiseReducer implements a simple spectral subtraction noise reduction
type NoiseReducer struct {
	// Configuration
	noiseFloor        float32 // Estimated noise floor level
	attenuationFactor float32 // Noise attenuation factor
	sampleRate        int     // Audio sample rate
	frameSize         int     // Size of audio frames in samples
	bytesPerSample    int     // Bytes per sample (typically 2 for PCM16)

	// State
	enabled               bool      // Whether noise reduction is enabled
	noiseProfile          []float32 // Noise spectral profile
	profileInitialized    bool      // Whether noise profile has been initialized
	noiseEstimationFrames int       // Number of frames to use for initial noise estimation
	framesProcessed       int       // Count of frames processed for noise estimation

	// For frequency domain processing
	fftSize int // FFT size (power of 2, typically 2x frameSize)

	// Lock for thread safety
	mu sync.Mutex
}

func DefaultNoiseReducer() *NoiseReducer {
	config := ProcessingConfig{
		// Voice Activity Detection
		EnableVAD:    true,
		VADThreshold: 0.02, // 2% of max energy
		VADHoldTime:  20,   // Hold voice detection for 20 frames (400ms at 50fps)

		// Noise Reduction
		EnableNoiseReduction: false,
		NoiseFloor:           0.01, // % of max signal
		NoiseAttenuationDB:   2.0,  // dB attenuation

		// Multi-channel Support
		ChannelCount: 1,    // Default to mono
		MixChannels:  true, // Mix channels by default

		// General Processing
		SampleRate: 48000, // 48kHz
		FrameSize:  960,   // 20ms at 48kHz
		BufferSize: 2048,  // Processing buffer size
	}
	return NewNoiseReducer(config)
}

// NewNoiseReducer creates a new noise reduction processor
func NewNoiseReducer(config ProcessingConfig) *NoiseReducer {
	fftSize := 512 // Power of 2, larger than typical frame size

	// Convert dB attenuation to linear factor
	attenuationFactor := float32(math.Pow(10, float64(-config.NoiseAttenuationDB/20.0)))

	return &NoiseReducer{
		noiseFloor:        config.NoiseFloor,
		attenuationFactor: attenuationFactor,
		sampleRate:        config.SampleRate,
		frameSize:         config.FrameSize,
		bytesPerSample:    2, // Assume 16-bit PCM

		enabled:               config.EnableNoiseReduction,
		noiseProfile:          make([]float32, fftSize/2+1), // Half FFT size + 1 for real signal
		profileInitialized:    false,
		noiseEstimationFrames: 30, // Use 30 frames (600ms at 20ms frames) for initial noise profile
		framesProcessed:       0,

		fftSize: fftSize,
	}
}

// Process implements AudioProcessor interface
// This is a simplified spectral subtraction implementation
func (nr *NoiseReducer) Process(samples []float32) []float32 {
	if nr.profileInitialized {
		return nr.process(samples)
	}
	n := len(samples)
	denoised := make([]float32, 0, n)
	for i := 0; i < n; i += nr.frameSize {
		end := i + nr.frameSize
		if end >= n {
			end = n
		}
		frame := nr.process(samples[i:end])
		denoised = append(denoised, frame...)
	}
	return denoised
}

func (nr *NoiseReducer) process(samples []float32) []float32 {
	if !nr.enabled {
		return samples
	}

	nr.mu.Lock()
	defer nr.mu.Unlock()

	// For simplicity in this implementation, we'll use a time-domain approach
	// rather than a full FFT-based spectral subtraction

	// If still building noise profile
	if !nr.profileInitialized && nr.framesProcessed < nr.noiseEstimationFrames {
		nr.updateNoiseProfile(samples)
		nr.framesProcessed++

		if nr.framesProcessed >= nr.noiseEstimationFrames {
			nr.profileInitialized = true
		}

		// During noise profile building, return original data
		return samples
	}

	// Process each sample with noise reduction
	processedSamples := make([]float32, len(samples))
	for i, sample := range samples {
		// Simple noise gate with smoothing
		if float32(math.Abs(float64(sample))) < nr.noiseFloor {
			// Attenuate noise
			processedSamples[i] = sample * nr.attenuationFactor
		} else {
			// Keep signal above noise floor
			// Apply soft transition at the threshold for smoother results
			ratio := float32(math.Min(1.0, (math.Abs(float64(sample))-float64(nr.noiseFloor))/float64(nr.noiseFloor*2)))
			attenuation := nr.attenuationFactor + (1.0-nr.attenuationFactor)*ratio
			processedSamples[i] = sample * attenuation
		}
	}
	return processedSamples
}

// updateNoiseProfile analyzes the audio to estimate the noise profile
func (nr *NoiseReducer) updateNoiseProfile(samples []float32) {
	// Calculate energy
	totalEnergy := float32(0.0)
	for _, sample := range samples {
		totalEnergy += sample * sample
	}
	avgEnergy := totalEnergy / float32(len(samples))

	// Update noise floor estimate with exponential moving average
	// Use slower adaptation for noise floor to avoid adapting to speech
	nr.noiseFloor = 0.9*nr.noiseFloor + 0.1*float32(math.Sqrt(float64(avgEnergy)))
}

// Reset implements AudioProcessor interface
func (nr *NoiseReducer) Reset() {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	nr.profileInitialized = false
	nr.framesProcessed = 0

	// Reset noise profile
	for i := range nr.noiseProfile {
		nr.noiseProfile[i] = 0.0
	}
}

// Close implements AudioProcessor interface
func (nr *NoiseReducer) Close() error {
	return nil
}

// SetEnabled enables or disables noise reduction
func (nr *NoiseReducer) SetEnabled(enabled bool) {
	nr.mu.Lock()
	defer nr.mu.Unlock()
	nr.enabled = enabled
}

// GetNoiseFloor returns the current estimated noise floor
func (nr *NoiseReducer) GetNoiseFloor() float32 {
	nr.mu.Lock()
	defer nr.mu.Unlock()
	return nr.noiseFloor
}
