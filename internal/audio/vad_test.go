package audio

import (
	"reflect"
	"testing"
)

func TestVADEnergyCalculation(t *testing.T) {
	config := ProcessingConfig{
		SampleRate:   8000,
		FrameSize:    160,
		VADThreshold: 0.01,
		BufferSize:   2048,
	}
	vad := NewVoiceActivityDetector(config)

	// Create a silent frame (all zeros)
	silence := make([]byte, 320) // 160 samples * 2 bytes
	energy := vad.calculateEnergy(silence)
	if energy != 0.0 {
		t.Errorf("energy = %f; want 0", energy)
	}

	// Create a "loud" frame (max amplitude)
	loud := make([]byte, 320)
	for i := 0; i < 320; i += 2 {
		loud[i] = 0xFF   // Little endian low byte
		loud[i+1] = 0x7F // Little endian high byte (~32767)
	}
	energy = vad.calculateEnergy(loud)
	if energy <= 0.9 {
		t.Errorf("energy = %f; want > 0.9", energy)
	}
}

func TestVADThresholdAndHold(t *testing.T) {
	config := ProcessingConfig{
		SampleRate:   8000,
		FrameSize:    160,
		VADThreshold: 0.1, // Set a distinct threshold
		VADHoldTime:  2,   // Hold for 2 frames
		BufferSize:   2048,
	}
	vad := NewVoiceActivityDetector(config)

	// 1. Initial State: Silence
	silence := make([]byte, 320)
	_, _ = vad.Process(silence)
	if vad.IsVoiceActive() == true {
		t.Errorf("Should not detect voice in initial silence")
	}

	// 2. Active Speech: High energy
	loud := make([]byte, 320)
	for i := 0; i < 320; i += 2 {
		loud[i] = 0x00
		loud[i+1] = 0x40 // ~16384 (Half valid range) => ~0.25 energy
	}
	_, _ = vad.Process(loud)
	if vad.IsVoiceActive() == false {
		t.Errorf("Should detect voice for loud frame")
	}
	if vad.holdCounter != 2 {

		t.Errorf("Hold counter should be set to holdTime")
	}

	// 3. Silence (Hold Period Frame 1)
	_, _ = vad.Process(silence)
	if vad.IsVoiceActive() == false {
		t.Errorf("Should still be active in hold period (1/2)")
	}
	if vad.holdCounter != 1 {
		t.Errorf("Hold counter should decrement")
	}

	// 4. Silence (Hold Period Frame 2)
	_, _ = vad.Process(silence)
	if vad.IsVoiceActive() == false {
		t.Errorf("Should still be active in hold period (2/2)")
	}
	if vad.holdCounter != 0 {
		t.Errorf("Hold counter should reach 0")
	}

	// 5. Silence (After Hold)
	_, _ = vad.Process(silence)
	if vad.IsVoiceActive() == true {
		t.Errorf("Should be inactive after hold period expires")
	}
}

func TestComfortNoiseGeneration(t *testing.T) {
	config := ProcessingConfig{
		SampleRate:   8000,
		FrameSize:    160,
		VADThreshold: 0.5,
		BufferSize:   2048,
	}
	vad := NewVoiceActivityDetector(config)
	vad.SetSilenceSuppression(true)
	vad.noiseFloor = 0.1 // Set a known noise floor

	// Process silence
	silence := make([]byte, 320)
	output, err := vad.Process(silence)
	if err != nil {
		t.Errorf("Error processing silence: %v", err)
	}

	// Should return comfort noise
	if reflect.DeepEqual(silence, output) == true {
		t.Errorf("Output should not be pure silence")
	}
	if len(output) != 16*2 {
		t.Errorf("Comfort noise length should be 16 samples (32 bytes)")
	}

	// Check that comfort noise is not empty/zero
	isZero := true
	for _, b := range output {
		if b != 0 {
			isZero = false
			break
		}
	}
	if isZero {
		t.Errorf("Comfort noise should have non-zero content")
	}
}

func TestVADReset(t *testing.T) {
	config := ProcessingConfig{
		SampleRate:   8000,
		FrameSize:    160,
		VADThreshold: 0.1,
	}
	vad := NewVoiceActivityDetector(config)

	// Simulate activity
	loud := make([]byte, 320)
	for i := 0; i < 320; i += 2 {
		loud[i+1] = 0x40
	}
	vad.Process(loud)
	if vad.IsVoiceActive() == false {
		t.Errorf("Should detect voice in loud frame")
	}

	// Reset
	vad.Reset()

	// Verify state is reset
	if vad.IsVoiceActive() == true {
		t.Errorf("Voice active state should be reset")
	}
	if vad.avgEnergy != 0.0 {
		t.Errorf("Average energy should be reset")
	}
	if vad.holdCounter != 0 {
		t.Errorf("Hold counter should reach 0")
	}
}
