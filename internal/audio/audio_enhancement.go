package audio

import (
	"log"
	"math"
	"sync"

	"github.com/CoyAce/apm"
)

// 常量定义
const (
	SampleRate = 48000 // 采样率
	FrameSize  = 480   // 帧大小：48000 * 10 / 1000 = 480
)

// EnhancementConfig contains configuration for audio enhancement
type EnhancementConfig struct {
	// AGC (Automatic Gain Control) settings
	AGC AGCConfig

	// Echo cancellation settings
	ApmConfig apm.Config

	// Dynamic range compression
	Compression CompressionConfig

	// Equalizer settings
	Equalizer EqualizerConfig

	// De-esser settings (reduces sibilance)
	DeEsser DeEsserConfig
}

// AGCConfig contains Automatic Gain Control configuration
type AGCConfig struct {
	Enabled bool

	// Target level in dBFS (typically -20 to -10)
	TargetLevel float32

	// Maximum gain in dB (typically 20-30)
	MaxGain float32

	// Minimum gain in dB (typically -20 to 0)
	MinGain float32

	// Attack time in milliseconds (how fast to increase gain)
	AttackTime float32

	// Release time in milliseconds (how fast to decrease gain)
	ReleaseTime float32

	// Noise gate threshold in dB (silence detection)
	NoiseGateThreshold float32

	// Hold time in milliseconds (prevent rapid gain changes)
	HoldTime float32

	SampleRate float32
}

// CompressionConfig contains dynamic range compression settings
type CompressionConfig struct {
	Enabled bool

	// Threshold in dB (where compression starts)
	Threshold float32

	// Compression ratio (e.g., 4:1)
	Ratio float32

	// Knee width in dB (smooth transition)
	Knee float32

	// Attack time in milliseconds
	AttackTime float32

	// Release time in milliseconds
	ReleaseTime float32

	// Makeup gain in dB
	MakeupGain float32

	SampleRate int
}

// EqualizerConfig contains equalizer settings
type EqualizerConfig struct {
	Enabled bool

	// Frequency bands (Hz) and their gains (dB)
	Bands []EqualizerBand

	// Pre-amplification in dB
	PreAmp float32

	SampleRate int
}

// EqualizerBand represents a single equalizer band
type EqualizerBand struct {
	Frequency float32 // Center frequency in Hz
	Gain      float32 // Gain in dB
	Q         float32 // Q factor (bandwidth)
}

// DeEsserConfig contains de-esser configuration
type DeEsserConfig struct {
	Enabled bool

	// Frequency range for sibilance detection (Hz)
	FrequencyMin float32
	FrequencyMax float32

	// Threshold in dB
	Threshold float32

	// Reduction amount (0.0-1.0)
	Reduction  float32
	SampleRate int
}

func EchoCancellationEnhancer() *Enhancer {
	config := DefaultEnhancementConfig()
	config.ApmConfig = apm.Config{CaptureChannels: 1, RenderChannels: 1, EchoCancellation: apm.EchoCancellationConfig{Enabled: true}}
	return NewEnhancer(config)
}

func DefaultAudioEnhancer() *Enhancer {
	config := DefaultEnhancementConfig()
	return NewEnhancer(config)
}

// DefaultEnhancementConfig returns default audio enhancement configuration
func DefaultEnhancementConfig() *EnhancementConfig {
	return &EnhancementConfig{
		AGC: AGCConfig{
			Enabled:            true,
			SampleRate:         48000,
			TargetLevel:        -23.0, // 标准语音输出电平
			MaxGain:            12.0,  // 轻度增益补偿
			MinGain:            -30.0, // 足够衰减大声信号
			AttackTime:         20.0,  // 平滑起音
			ReleaseTime:        400.0, // 缓慢释放，避免呼吸声
			NoiseGateThreshold: -40.0, // 合理噪声门限
			HoldTime:           50.0,  // 适当保持
		},
		Compression: CompressionConfig{
			Enabled:     false,
			SampleRate:  48000,
			Threshold:   -20.0,
			Ratio:       4.0,
			Knee:        2.0,
			AttackTime:  5.0,
			ReleaseTime: 50.0,
			MakeupGain:  0.0,
		},
		Equalizer: EqualizerConfig{
			Enabled:    true,
			SampleRate: 48000,
			Bands: []EqualizerBand{
				// Low-frequency management: 1 band
				{Frequency: 120, Gain: -1.0, Q: 1.0},

				// Mid-frequency shaping: 3 core bands
				{Frequency: 300, Gain: 1.0, Q: 0.7},  // Warmth
				{Frequency: 1000, Gain: 0.5, Q: 0.7}, // Presence
				{Frequency: 2500, Gain: 1.5, Q: 0.7}, // Clarity (Key!)

				// High-frequency coordination: 1 band working with DeEsser
				{Frequency: 5000, Gain: -0.5, Q: 1.5}, // Mild sibilance control

				// Optional: Air recovery (if sound is too dull)
				{Frequency: 8000, Gain: 0.0, Q: 0.7}, // Neutral or slight boost
			},
			PreAmp: 0.0,
		},
		DeEsser: DeEsserConfig{
			Enabled:      true,
			SampleRate:   48000,
			FrequencyMin: 4500.0, // Focus on core sibilance range
			FrequencyMax: 7000.0, // Avoid interfering with 8000Hz air band
			Threshold:    -25.0,  // Reduce false triggering
			Reduction:    0.3,    // Gentle compression
		},
	}
}

// Enhancer provides comprehensive audio enhancement
type Enhancer struct {
	config *EnhancementConfig
	mu     sync.RWMutex

	preamp         *Preamp
	highPassFilter *HighPassFilter
	nr             *NoiseReducer

	// AGC components
	agc *AutomaticGainControl

	// Echo cancellation components
	processor *apm.Processor

	// Compressor
	compressor *DynamicRangeCompressor

	// Equalizer
	equalizer *ParametricEqualizer

	// De-esser
	deesser *DeEsser

	// Processing metrics
	metrics EnhancementMetrics
}

// EnhancementMetrics tracks enhancement metrics
type EnhancementMetrics struct {
	InputLevel      float32
	OutputLevel     float32
	CurrentGain     float32
	CompressionGain float32
	ProcessedFrames uint64
	ERLE            float32
	Delay           int
}

// NewEnhancer creates a new audio enhancer
func NewEnhancer(config *EnhancementConfig) *Enhancer {
	if config == nil {
		config = DefaultEnhancementConfig()
	}
	ae := &Enhancer{
		config:         config,
		preamp:         NewPreamp(),
		highPassFilter: NewHighPassFilter(80, config.AGC.SampleRate),
		nr:             DefaultNoiseReducer(),
		agc:            NewAutomaticGainControl(&config.AGC),
		compressor:     NewDynamicRangeCompressor(&config.Compression),
		equalizer:      NewParametricEqualizer(&config.Equalizer),
		deesser:        NewDeEsser(&config.DeEsser),
	}
	if config.ApmConfig.EchoCancellation.Enabled {
		processor, err := apm.New(config.ApmConfig)
		if err != nil {
			log.Printf("Can't create processor: %v", err)
		}
		ae.processor = processor
	}

	return ae
}

func (ae *Enhancer) Initialize() {
	if ae.processor != nil {
		ae.processor.Initialize()
	}
}

// AddFarEnd - 单独添加远端信号（用于异步处理）
func (ae *Enhancer) AddFarEnd(farEnd []int16) {
	if len(farEnd) != FrameSize && ae.processor == nil {
		return
	}
	err := ae.processor.ProcessRenderInt16(farEnd)
	if err != nil {
		log.Printf("Enhancement processor error: %v", err)
	}
}

// ProcessAudio applies all enhancement stages to audio
func (ae *Enhancer) ProcessAudio(samples []float32) ([]float32, error) {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	ae.metrics.ProcessedFrames++
	// Stage 1: HighPass Filter
	ae.highPassFilter.ProcessBatch(samples)
	// Stage 2: Preamp
	output, info := ae.preamp.Process(samples)
	if info == nil || info.Silent {
		return output, nil
	}
	// Track input level
	ae.metrics.InputLevel = info.RMS

	// Stage 3: Echo cancellation (should be first)
	if ae.config.ApmConfig.EchoCancellation.Enabled {
		out, err := ae.processor.ProcessCapture(output)
		if err == nil {
			output = out
		}
		ae.metrics.ERLE = float32(ae.processor.GetStats().EchoReturnLossEnhancement)
	}

	// Stage 4: Noise gate (part of AGC)
	if ae.config.AGC.Enabled {
		output = ae.agc.ApplyNoiseGate(output)
	}

	// Stage 5: Noise Reducer
	if ae.nr.enabled {
		output = ae.nr.Process(output)
	}

	// Stage 6: Equalizer
	if ae.config.Equalizer.Enabled {
		output = ae.equalizer.Process(output)
	}

	// Stage 7: De-esser
	if ae.config.DeEsser.Enabled {
		output = ae.deesser.Process(output)
	}

	// Stage 8: AGC
	if ae.config.AGC.Enabled {
		output, ae.metrics.CurrentGain = ae.agc.Process(output)
	}

	// Stage 9: Compression
	if ae.config.Compression.Enabled {
		output, ae.metrics.CompressionGain = ae.compressor.Process(output)
	}

	// Track output level
	ae.metrics.OutputLevel = calculateRMS(output)

	return output, nil
}

// GetMetrics returns current enhancement metrics
func (ae *Enhancer) GetMetrics() EnhancementMetrics {
	ae.mu.RLock()
	defer ae.mu.RUnlock()
	return ae.metrics
}

// AutomaticGainControl implements AGC
type AutomaticGainControl struct {
	config *AGCConfig
	mu     sync.Mutex

	currentGain   float32
	targetLevel   float32
	envelope      float32
	gateThreshold float32
	holdCounter   int

	// Time constants
	attackCoeff  float32
	releaseCoeff float32
}

// NewAutomaticGainControl creates a new AGC processor
func NewAutomaticGainControl(config *AGCConfig) *AutomaticGainControl {
	agc := &AutomaticGainControl{
		config:        config,
		currentGain:   1.0,
		targetLevel:   dbToLinear(config.TargetLevel),
		gateThreshold: dbToLinear(config.NoiseGateThreshold),
	}

	// Calculate time constants
	agc.attackCoeff = float32(1.0 - math.Exp(float64(-1.0/(config.AttackTime*config.SampleRate/1000.0))))
	agc.releaseCoeff = float32(1.0 - math.Exp(float64(-1.0/(config.ReleaseTime*config.SampleRate/1000.0))))

	return agc
}

// Process applies AGC to audio samples
func (agc *AutomaticGainControl) Process(samples []float32) ([]float32, float32) {
	agc.mu.Lock()
	defer agc.mu.Unlock()

	if !agc.config.Enabled {
		return samples, 1.0
	}

	output := make([]float32, len(samples))

	for i, sample := range samples {
		// Update envelope follower
		absSample := float32(math.Abs(float64(sample)))
		if absSample > agc.envelope {
			agc.envelope += agc.attackCoeff * (absSample - agc.envelope)
		} else {
			agc.envelope += agc.releaseCoeff * (absSample - agc.envelope)
		}

		// Calculate desired gain
		desiredGain := float32(1.0)
		if agc.envelope > 0.001 {
			desiredGain = agc.targetLevel / agc.envelope
		}

		// Limit gain
		if desiredGain > dbToLinear(agc.config.MaxGain) {
			desiredGain = dbToLinear(agc.config.MaxGain)
		} else if desiredGain < dbToLinear(agc.config.MinGain) {
			desiredGain = dbToLinear(agc.config.MinGain)
		}

		// Smooth gain changes
		if desiredGain > agc.currentGain {
			agc.currentGain += agc.attackCoeff * (desiredGain - agc.currentGain)
		} else {
			agc.currentGain += agc.releaseCoeff * (desiredGain - agc.currentGain)
		}

		// Apply gain
		output[i] = sample * agc.currentGain

		// Prevent clipping
		if output[i] > 0.95 {
			output[i] = 0.95
		} else if output[i] < -0.95 {
			output[i] = -0.95
		}
	}

	return output, agc.currentGain
}

// ApplyNoiseGate applies noise gate to silence low-level noise
func (agc *AutomaticGainControl) ApplyNoiseGate(samples []float32) []float32 {
	agc.mu.Lock()
	defer agc.mu.Unlock()

	output := make([]float32, len(samples))

	for i, sample := range samples {
		level := float32(math.Abs(float64(sample)))

		if level < agc.gateThreshold {
			// Below threshold - apply gate
			if agc.holdCounter > 0 {
				agc.holdCounter--
				output[i] = sample // Hold period
			} else {
				output[i] = sample * 0.1 // Attenuate
			}
		} else {
			// Above threshold - pass through
			agc.holdCounter = int(agc.config.HoldTime * 8) // Reset hold counter
			output[i] = sample
		}
	}

	return output
}

// DynamicRangeCompressor implements audio compression
type DynamicRangeCompressor struct {
	config       *CompressionConfig
	envelope     float32
	attackCoeff  float32
	releaseCoeff float32
}

// NewDynamicRangeCompressor creates a new compressor
func NewDynamicRangeCompressor(config *CompressionConfig) *DynamicRangeCompressor {

	return &DynamicRangeCompressor{
		config:       config,
		attackCoeff:  1.0 - float32(math.Exp(-1.0/(float64(config.AttackTime)*float64(config.SampleRate)/1000.0))),
		releaseCoeff: 1.0 - float32(math.Exp(-1.0/(float64(config.ReleaseTime)*float64(config.SampleRate)/1000.0))),
	}
}

// Process applies dynamic range compression
func (drc *DynamicRangeCompressor) Process(samples []float32) ([]float32, float32) {
	if !drc.config.Enabled {
		return samples, 1.0
	}

	output := make([]float32, len(samples))
	avgGain := float32(0.0)

	for i, sample := range samples {
		// Update envelope
		level := float32(math.Abs(float64(sample)))
		if level > drc.envelope {
			drc.envelope += drc.attackCoeff * (level - drc.envelope)
		} else {
			drc.envelope += drc.releaseCoeff * (level - drc.envelope)
		}

		// Calculate gain reduction
		gainReduction := float32(1.0)
		levelDb := linearToDb(drc.envelope)

		if levelDb > drc.config.Threshold {
			// Apply compression
			excess := levelDb - drc.config.Threshold

			// Apply soft knee
			if excess < drc.config.Knee {
				ratio := 1.0 + (drc.config.Ratio-1.0)*(excess/drc.config.Knee)*(excess/drc.config.Knee)
				excess = excess / ratio
			} else {
				excess = excess / drc.config.Ratio
			}

			gainReduction = dbToLinear(-excess)
		}

		// Apply gain reduction and makeup gain
		gain := gainReduction * dbToLinear(drc.config.MakeupGain)
		output[i] = sample * gain
		avgGain += gain
	}

	return output, avgGain / float32(len(samples))
}

// ParametricEqualizer implements multi-band parametric EQ
type ParametricEqualizer struct {
	config *EqualizerConfig
	bands  []*BiquadFilter
}

// NewParametricEqualizer creates a new equalizer
func NewParametricEqualizer(config *EqualizerConfig) *ParametricEqualizer {
	eq := &ParametricEqualizer{
		config: config,
		bands:  make([]*BiquadFilter, len(config.Bands)),
	}

	// Create biquad filters for each band
	for i, band := range config.Bands {
		eq.bands[i] = NewBiquadFilter(band.Frequency, band.Gain, band.Q, float32(config.SampleRate))
	}

	return eq
}

// Process applies equalization
func (eq *ParametricEqualizer) Process(samples []float32) []float32 {
	if !eq.config.Enabled {
		return samples
	}

	output := make([]float32, len(samples))
	copy(output, samples)

	// Apply pre-amplification
	if eq.config.PreAmp != 0 {
		preAmpGain := dbToLinear(eq.config.PreAmp)
		for i := range output {
			output[i] *= preAmpGain
		}
	}

	// Apply each band
	for _, band := range eq.bands {
		output = band.Process(output)
	}

	return output
}

// BiquadFilter implements a second-order IIR filter
type BiquadFilter struct {
	a0, a1, a2 float32 // Feedforward coefficients
	b1, b2     float32 // Feedback coefficients
	x1, x2     float32 // Input delay line
	y1, y2     float32 // Output delay line
}

// NewBiquadFilter creates a peaking EQ biquad filter
func NewBiquadFilter(frequency, gain, q, sampleRate float32) *BiquadFilter {
	omega := float64(2.0 * math.Pi * frequency / sampleRate)
	alpha := float32(math.Sin(omega)) / (2.0 * q)
	A := float32(math.Sqrt(float64(dbToLinear(gain))))

	// Peaking EQ coefficients
	b0 := 1.0 + alpha*A
	b1 := -2.0 * float32(math.Cos(omega))
	b2 := 1.0 - alpha*A
	a0 := 1.0 + alpha/A
	a1 := -2.0 * float32(math.Cos(omega))
	a2 := 1.0 - alpha/A

	// Normalize
	return &BiquadFilter{
		a0: b0 / a0,
		a1: b1 / a0,
		a2: b2 / a0,
		b1: a1 / a0,
		b2: a2 / a0,
	}
}

// Process applies the biquad filter
func (bf *BiquadFilter) Process(samples []float32) []float32 {
	output := make([]float32, len(samples))

	for i, x0 := range samples {
		// Direct Form II
		y0 := bf.a0*x0 + bf.a1*bf.x1 + bf.a2*bf.x2 - bf.b1*bf.y1 - bf.b2*bf.y2

		// Update delay lines
		bf.x2 = bf.x1
		bf.x1 = x0
		bf.y2 = bf.y1
		bf.y1 = y0

		output[i] = y0
	}

	return output
}

// DeEsser reduces sibilance in audio
type DeEsser struct {
	config       *DeEsserConfig
	detector     *BiquadFilter
	envelope     float32
	attackCoeff  float32
	releaseCoeff float32
}

// NewDeEsser creates a new de-esser
func NewDeEsser(config *DeEsserConfig) *DeEsser {
	centerFreq := (config.FrequencyMin + config.FrequencyMax) / 2
	bandwidth := config.FrequencyMax - config.FrequencyMin
	q := centerFreq / bandwidth

	return &DeEsser{
		config:       config,
		detector:     NewBiquadFilter(centerFreq, 0, q, float32(config.SampleRate)),
		attackCoeff:  0.99,
		releaseCoeff: 0.999,
	}
}

// Process applies de-essing
func (de *DeEsser) Process(samples []float32) []float32 {
	if !de.config.Enabled {
		return samples
	}

	// Detect sibilance
	detected := de.detector.Process(samples)
	output := make([]float32, len(samples))

	for i := range samples {
		// Update envelope of detected signal
		level := float32(math.Abs(float64(detected[i])))
		if level > de.envelope {
			de.envelope = de.attackCoeff*de.envelope + (1-de.attackCoeff)*level
		} else {
			de.envelope = de.releaseCoeff*de.envelope + (1-de.releaseCoeff)*level
		}

		// Calculate reduction
		reduction := float32(1.0)
		if linearToDb(de.envelope) > de.config.Threshold {
			reduction = 1.0 - de.config.Reduction
		}

		// Apply reduction only to high frequencies
		highFreq := detected[i]
		lowFreq := samples[i] - highFreq
		output[i] = lowFreq + highFreq*reduction
	}

	return output
}

// Helper function to calculate RMS
func calculateRMS(samples []float32) float32 {
	sum := 0.0
	for _, s := range samples {
		sum += float64(s * s)
	}
	return float32(math.Sqrt(sum / float64(len(samples))))
}
