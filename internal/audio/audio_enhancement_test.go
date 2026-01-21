package audio

import (
	"math"
	"reflect"
	"testing"
)

func TestAudioEnhancer_NewAudioEnhancer(t *testing.T) {
	config := DefaultEnhancementConfig()

	enhancer := NewEnhancer(config)
	if enhancer == nil {
		t.Error("enhancer is nil")
	}
	if enhancer.agc == nil {
		t.Error("enhancer agc is nil")
	}
	if enhancer.echo == nil {
		t.Error("enhancer echo is nil")
	}
	if enhancer.compressor == nil {
		t.Error("enhancer compressor is nil")
	}
}

func TestAudioEnhancer_ProcessAudio(t *testing.T) {
	config := &EnhancementConfig{
		AGC: AGCConfig{
			Enabled:     true,
			TargetLevel: -20,
		},
		Compression: CompressionConfig{
			Enabled:   true,
			Threshold: -20,
		},
	}

	enhancer := NewEnhancer(config)

	// Create test audio with varying amplitude
	samples := make([]float32, 1024)
	for i := range samples {
		// Create signal with varying amplitude
		amplitude := 0.1 + 0.8*float64(i)/float64(len(samples))
		samples[i] = float32(amplitude * math.Sin(2*math.Pi*440*float64(i)/8000))
	}

	processed, err := enhancer.ProcessAudio(samples)
	if err != nil {
		t.Error(err)
	}
	if processed == nil {
		t.Error("processed is nil")
	}
	if len(processed) != len(samples) {
		t.Error("processed and samples are not equal")
	}

	// Check that processing was applied
	different := false
	for i := range samples {
		if samples[i] != processed[i] {
			different = true
			break
		}
	}
	if different != true {
		t.Error("Processed audio should differ from input")
	}
}

func TestAudioEnhancer_ProcessBasic(t *testing.T) {
	config := DefaultEnhancementConfig()
	config.AGC.Enabled = true
	config.EchoCancellation.Enabled = false // Disable echo cancellation for basic test
	config.Compression.Enabled = true

	enhancer := NewEnhancer(config)

	// Create test signal
	samples := make([]float32, 1024)
	for i := range samples {
		samples[i] = float32(0.5 * math.Sin(2*math.Pi*440*float64(i)/8000))
	}

	processed, err := enhancer.ProcessAudio(samples)
	if err != nil {
		t.Error(err)
	}
	if processed == nil {
		t.Error("processed is nil")
	}
	if len(processed) != len(samples) {
		t.Error("processed and samples are not equal")
	}
}

func TestAudioEnhancer_AllFeaturesEnabled(t *testing.T) {
	config := &EnhancementConfig{
		AGC: AGCConfig{
			Enabled:     true,
			TargetLevel: -20,
		},
		EchoCancellation: EchoCancellationConfig{
			Enabled:      true,
			FilterLength: 256,
		},
		Compression: CompressionConfig{
			Enabled:   true,
			Threshold: -20,
		},
	}

	enhancer := NewEnhancer(config)

	// Complex test signal
	samples := make([]float32, 1024)
	for i := range samples {
		// Mix of frequencies with varying amplitude
		amplitude := 0.2 + 0.6*float64(i%200)/200
		low := amplitude * 0.3 * math.Sin(2*math.Pi*200*float64(i)/8000)
		mid := amplitude * 0.4 * math.Sin(2*math.Pi*1000*float64(i)/8000)
		high := amplitude * 0.3 * math.Sin(2*math.Pi*3000*float64(i)/8000)
		samples[i] = float32(low + mid + high)
	}

	processed, err := enhancer.ProcessAudio(samples)
	if err != nil {
		t.Error(err)
	}
	if processed == nil {
		t.Error("processed is nil")
	}
	if len(processed) != len(samples) {
		t.Error("processed and samples are not equal")
	}
	if reflect.DeepEqual(samples, processed) {
		t.Error("samples are not processed")
	}
}

func BenchmarkAudioEnhancer_ProcessAudio(b *testing.B) {
	config := &EnhancementConfig{
		AGC:              AGCConfig{Enabled: true},
		EchoCancellation: EchoCancellationConfig{Enabled: true},
		Compression:      CompressionConfig{Enabled: true},
	}

	enhancer := NewEnhancer(config)
	samples := make([]float32, 1024)
	for i := range samples {
		samples[i] = float32(math.Sin(2 * math.Pi * 1000 * float64(i) / 8000))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enhancer.ProcessAudio(samples)
	}
}
