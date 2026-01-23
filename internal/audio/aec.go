package audio

import (
	"math"
	"math/rand"
	"sync"
)

// EchoCanceller implements acoustic echo cancellation
type EchoCanceller struct {
	config *EchoCancellationConfig
	mu     sync.Mutex

	// Adaptive filter coefficients
	filterCoeffs []float32
	filterBuffer []float32

	// Reference signal buffer (far-end)
	referenceBuffer []float32

	// Error signal for adaptation
	errorSignal []float32

	// Double-talk detector
	nearEndPower float32
	farEndPower  float32
	doubleTalk   bool

	// Metrics
	echoReduction float32
	ERLE          float32
}

// NewEchoCanceller creates a new echo canceller
func NewEchoCanceller(config *EchoCancellationConfig) *EchoCanceller {
	filterLen := int(config.FilterLength * float32(config.SampleRate/1000)) // 48 samples per ms at 48kHz

	ec := &EchoCanceller{
		config:          config,
		filterCoeffs:    make([]float32, filterLen),
		filterBuffer:    make([]float32, filterLen),
		referenceBuffer: make([]float32, filterLen),
		errorSignal:     make([]float32, filterLen),
	}
	ec.initializeFilterCoefficients()
	return ec
}

func (ec *EchoCanceller) initializeFilterCoefficients() {
	// 添加小的随机值
	for i := range ec.filterCoeffs {
		ec.filterCoeffs[i] += (rand.Float32() - 0.5) * 0.01
	}
}

// Process removes echo from audio signal
func (ec *EchoCanceller) Process(reference, samples []float32) []float32 {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	if !ec.config.Enabled || len(reference) == 0 {
		return samples
	}

	output := make([]float32, len(samples))

	// Update reference buffer
	if len(reference) > 0 {
		ec.updateReferenceBuffer(reference)
	}
	for i, sample := range samples {
		// Estimate echo using adaptive filter
		echoEstimate := ec.estimateEcho()

		// Subtract estimated echo
		errorSignal := sample - echoEstimate

		// Detect double-talk
		ec.detectDoubleTalk(sample, echoEstimate)

		// Update filter coefficients (if not double-talk)
		if !ec.doubleTalk {
			ec.updateFilterCoefficients(errorSignal)
		}

		// Apply nonlinear processing
		processed := ec.applyNonlinearProcessing(errorSignal)

		// Add comfort noise
		processed = ec.addComfortNoise(processed)

		// Apply residual echo suppression
		output[i] = ec.suppressResidualEcho(processed, echoEstimate)
	}

	inputEnergy := calculateRMS(samples)
	outputEnergy := calculateRMS(output)

	// 平滑更新
	alpha := float32(0.1) // 平滑系数

	// 计算ERLE（dB）
	if outputEnergy > 0 {
		ERLE := 10 * float32(math.Log10(float64(inputEnergy/outputEnergy)))
		ec.ERLE = (1-alpha)*ec.ERLE + alpha*ERLE
	}

	return output
}

// updateReferenceBuffer updates the reference signal buffer
func (ec *EchoCanceller) updateReferenceBuffer(samples []float32) {
	if len(ec.referenceBuffer) == 0 {
		return
	}
	// Shift buffer and add new samples
	copy(ec.referenceBuffer[len(samples):], ec.referenceBuffer)
	copy(ec.referenceBuffer[0:], samples)
}

// estimateEcho estimates the echo signal
func (ec *EchoCanceller) estimateEcho() float32 {
	estimate := float32(0.0)
	for j := 0; j < len(ec.filterCoeffs); j++ {
		if j < len(ec.referenceBuffer) {
			estimate += ec.filterCoeffs[j] * ec.referenceBuffer[j]
		}
	}
	return estimate
}

// detectDoubleTalk detects simultaneous near-end and far-end speech
func (ec *EchoCanceller) detectDoubleTalk(nearEnd, farEnd float32) {
	// Update power estimates
	alpha := float32(0.99)
	ec.nearEndPower = alpha*ec.nearEndPower + (1-alpha)*nearEnd*nearEnd
	ec.farEndPower = alpha*ec.farEndPower + (1-alpha)*farEnd*farEnd

	// Detect double-talk
	if ec.farEndPower > 0 {
		ratio := ec.nearEndPower / ec.farEndPower
		ec.doubleTalk = ratio > ec.config.DoubleTalkThreshold
	} else {
		ec.doubleTalk = false
	}
}

// updateFilterCoefficients updates adaptive filter using NLMS algorithm
func (ec *EchoCanceller) updateFilterCoefficients(error float32) {
	// Calculate step size
	power := float32(0.0)
	for _, ref := range ec.referenceBuffer {
		power += ref * ref
	}

	if power > 0.001 {
		stepSize := ec.config.AdaptationRate / (power + 0.001)

		// Update coefficients
		for j := 0; j < len(ec.filterCoeffs); j++ {
			if j < len(ec.referenceBuffer) {
				ec.filterCoeffs[j] += stepSize * error * ec.referenceBuffer[j]
			}
		}
	}
}

// applyNonlinearProcessing applies NLP to remove residual echo
func (ec *EchoCanceller) applyNonlinearProcessing(signal float32) float32 {
	threshold := 0.01 * ec.config.NonlinearProcessing

	if float32(math.Abs(float64(signal))) < threshold {
		// Suppress small signals (likely residual echo)
		return signal * 0.1
	}

	return signal
}

// addComfortNoise adds comfort noise during suppression
func (ec *EchoCanceller) addComfortNoise(signal float32) float32 {
	noiseLevel := dbToLinear(ec.config.ComfortNoiseLevel)
	noise := float32(math.Sin(float64(ec.farEndPower*1000))) * noiseLevel
	return signal + noise
}

// suppressResidualEcho applies residual echo suppression
func (ec *EchoCanceller) suppressResidualEcho(signal, echoEstimate float32) float32 {
	if math.Abs(float64(echoEstimate)) > 0.001 {
		suppression := 1.0 - ec.config.ResidualSuppression*float32(math.Min(1.0, math.Abs(float64(echoEstimate))/0.1))
		return signal * suppression
	}
	return signal
}

// updateMetrics updates echo cancellation metrics
func (ec *EchoCanceller) updateMetrics(input, output float32) {
	if math.Abs(float64(input)) > 0.001 {
		ec.echoReduction = 1.0 - float32(math.Abs(float64(output))/math.Abs(float64(input)))
	}
}

// GetReduction returns current echo reduction amount
func (ec *EchoCanceller) GetReduction() float32 {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return ec.echoReduction
}
