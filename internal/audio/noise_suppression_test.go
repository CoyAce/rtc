package audio

import (
	"math"
	"testing"
)

func TestNoiseSuppressor_NewNoiseSuppressor(t *testing.T) {
	config := DefaultNoiseSuppressionConfig()

	ns := NewNoiseSuppressor(config)
	if ns == nil {
		t.Errorf("NewNoiseSuppressor() returned nil instance")
	}
	if ns.config == nil {
		t.Errorf("NewNoiseSuppressor() returned nil config")
	}
}

func TestNoiseSuppressor_ProcessFrame(t *testing.T) {
	config := DefaultNoiseSuppressionConfig()
	config.Enabled = true

	ns := NewNoiseSuppressor(config)

	t.Run("process silence", func(t *testing.T) {
		// Process silence (should update noise profile)
		silence := make([]float32, 512)
		processed := ns.ProcessFrame(silence)
		if processed == nil {
			t.Errorf("ProcessFrame() returned nil instance")
		}
		if len(processed) != 512 {
			t.Errorf("ProcessFrame() processed length should be 512")
		}

		// All samples should be near zero
		for _, sample := range processed {
			if sample < -0.01 || sample > 0.01 {
				t.Errorf("All samples should be near zero")
			}
		}
	})

	t.Run("process tone with noise", func(t *testing.T) {
		// Create a test signal with a tone and noise
		samples := make([]float32, 512)
		for i := range samples {
			// 1kHz tone at 8kHz sample rate
			tone := 0.5 * math.Sin(2*math.Pi*1000*float64(i)/8000)
			noise := 0.1 * (math.Sin(float64(i)*0.1) + math.Cos(float64(i)*0.3))
			samples[i] = float32(tone + noise)
		}

		processed := ns.ProcessFrame(samples)
		if processed == nil {
			t.Errorf("ProcessFrame() returned nil instance")
		}
		if len(processed) != 512 {
			t.Errorf("ProcessFrame() processed length should be 512")
		}

		// Verify some noise reduction occurred
		var originalEnergy, processedEnergy float32
		for i := range samples {
			originalEnergy += samples[i] * samples[i]
			processedEnergy += processed[i] * processed[i]
		}

		// Processed should have less energy due to noise reduction
		if processedEnergy >= originalEnergy {
			t.Errorf("Processed should have less energy due to noise reduction")
		}
	})
}

func TestNoiseSuppressor_ConcurrentProcessing(t *testing.T) {
	config := DefaultNoiseSuppressionConfig()
	ns := NewNoiseSuppressor(config)

	// Process multiple audio streams concurrently
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			samples := make([]float32, 512)
			for j := range samples {
				samples[j] = float32(0.1 * math.Sin(float64(j+id)))
			}

			processed := ns.ProcessFrame(samples)
			if processed == nil {
				t.Errorf("ProcessFrame() returned nil instance")
			}
			if len(processed) != 512 {
				t.Errorf("ProcessFrame() processed length should be 512")
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func BenchmarkNoiseSuppressor_ProcessFrame(b *testing.B) {
	config := DefaultNoiseSuppressionConfig()
	ns := NewNoiseSuppressor(config)
	samples := make([]float32, 512)
	for i := range samples {
		samples[i] = float32(0.5 * math.Sin(2*math.Pi*1000*float64(i)/8000))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ns.ProcessFrame(samples)
	}
}
