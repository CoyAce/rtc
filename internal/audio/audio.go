package audio

import (
	"context"
	"fmt"
	"io"
	"math"
	"strings"
	"unsafe"

	"github.com/CoyAce/opus/ogg"
	"github.com/gen2brain/malgo"
)

// StreamConfig describes the parameters for an audio stream.
// Default values will pick the defaults of the default device.
type StreamConfig struct {
	Format          malgo.FormatType
	Channels        int
	SampleRate      int
	DeviceType      malgo.DeviceType
	MalgoContext    malgo.Context
	CaptureDeviceID *malgo.DeviceID
}

func (config StreamConfig) asDeviceConfig(deviceType malgo.DeviceType) malgo.DeviceConfig {
	deviceConfig := malgo.DefaultDeviceConfig(deviceType)
	if config.Format != malgo.FormatUnknown {
		deviceConfig.Capture.Format = config.Format
		deviceConfig.Playback.Format = config.Format
	}
	if config.Channels != 0 {
		deviceConfig.Capture.Channels = uint32(config.Channels)
		deviceConfig.Playback.Channels = uint32(config.Channels)
	}
	if config.SampleRate != 0 {
		deviceConfig.SampleRate = uint32(config.SampleRate)
	}
	if config.DeviceType != 0 {
		deviceConfig.DeviceType = config.DeviceType
	}
	if config.CaptureDeviceID != nil {
		deviceConfig.Capture.DeviceID = config.CaptureDeviceID.Pointer()
	}
	return deviceConfig
}

// SetCaptureDeviceByName sets the capture DeviceID by matching device name.
// Returns true if found and set.
func (c *StreamConfig) SetCaptureDeviceByName(mctx *malgo.Context, name string) (bool, error) {
	if name == "" {
		return false, nil
	}
	devices, err := mctx.Devices(malgo.Capture)
	if err != nil {
		return false, err
	}
	for _, d := range devices {
		if strings.TrimSpace(d.Name()) == name {
			id := malgo.DeviceID(d.ID)
			c.CaptureDeviceID = &id
			return true, nil
		}
	}
	return false, nil
}

func stream(ctx context.Context, abortChan chan error, config StreamConfig, deviceCallbacks malgo.DeviceCallbacks) error {
	deviceConfig := config.asDeviceConfig(malgo.Capture)
	device, err := malgo.InitDevice(config.MalgoContext, deviceConfig, deviceCallbacks)
	if err != nil {
		return err
	}
	defer device.Uninit()

	err = device.Start()
	if err != nil {
		return err
	}

	ctxChan := ctx.Done()
	if ctxChan != nil {
		select {
		case <-ctxChan:
			err = ctx.Err()
		case err = <-abortChan:
		}
	} else {
		err = <-abortChan
	}

	return err
}

// ListCaptureDevices returns all available capture devices with their names and IDs.
func ListCaptureDevices() error {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return err
	}
	defer ctx.Uninit()
	mctx := &ctx.Context

	devs, err := mctx.Devices(malgo.Capture)
	if err != nil {
		return err
	}

	fmt.Println("Available capture devices:")
	for i, d := range devs {
		note := ""
		if d.IsDefault != 0 {
			note = "(DEFAULT)"
		}
		fmt.Printf("  %d: \"%s\" %s\n  ID: %s\n", i, d.Name(), note, d.ID.String())
	}
	if len(devs) == 0 {
		fmt.Println("  (none found)")
	}

	return nil
}

// Capture records incoming samples into the provided writer.
// The function initializes a capture device in the default context using
// provide stream configuration.
// Capturing will commence writing the samples to the writer until either the
// writer returns an error, or the context signals done.
func Capture(ctx context.Context, w io.Writer, config StreamConfig) error {
	config.DeviceType = malgo.Capture
	abortChan := make(chan error, 1)
	defer close(abortChan)
	aborted := false

	deviceCallbacks := malgo.DeviceCallbacks{
		Data: func(outputSamples, inputSamples []byte, frameCount uint32) {
			if aborted {
				return
			}
			_, err := w.Write(inputSamples)
			if err != nil {
				aborted = true
				abortChan <- err
			}
		},
	}

	return stream(ctx, abortChan, config, deviceCallbacks)
}

// Playback streams samples from a reader to the sound device.
// The function initializes a playback device in the default context using
// provide stream configuration.
// Playback will commence playing the samples provided from the reader until either the
// reader returns an error, or the context signals done.
func Playback(ctx context.Context, r io.Reader, config StreamConfig) error {
	config.DeviceType = malgo.Playback
	abortChan := make(chan error, 1)
	defer close(abortChan)
	aborted := false

	deviceCallbacks := malgo.DeviceCallbacks{
		Data: func(outputSamples, inputSamples []byte, frameCount uint32) {
			if aborted {
				return
			}
			if frameCount == 0 {
				return
			}

			read, err := io.ReadFull(r, outputSamples)
			if read <= 0 {
				if err != nil {
					aborted = true
					abortChan <- err
				}
				return
			}
		},
	}

	return stream(ctx, abortChan, config, deviceCallbacks)
}

func NewStreamConfig(maCtx *malgo.AllocatedContext, channels int) StreamConfig {
	return StreamConfig{
		Format:       malgo.FormatS16,
		Channels:     channels,
		SampleRate:   48000,
		MalgoContext: maCtx.Context,
	}
}

func ToFloat32(pcm []byte) []float32 {
	if len(pcm) == 0 {
		return []float32{}
	}
	size := len(pcm) / 4
	cap := cap(pcm) / 4

	header := struct {
		data unsafe.Pointer
		len  int
		cap  int
	}{
		data: unsafe.Pointer(&pcm[0]),
		len:  size,
		cap:  cap,
	}
	return *(*[]float32)(unsafe.Pointer(&header))
}

func dbToLinear(db float32) float32 {
	return float32(math.Pow(10, float64(db)/20.0))
}

func linearToDb(linear float32) float32 {
	if linear <= 0 {
		return -100.0
	}
	return 20 * float32(math.Log10(float64(linear)))
}

// Int16ToFloat32 - int16转float32 [-32768, 32767] -> [-1.0, 1.0]
func Int16ToFloat32(pcm []int16) []float32 {
	floats := make([]float32, len(pcm))
	for i, v := range pcm {
		floats[i] = float32(v) / 32768.0
	}
	return floats
}

func Int16BytesToFloat32(pcm []byte) []float32 {
	return Int16ToFloat32(ogg.ToInts(pcm))
}

// Float32ToInt16 - float32转int16 [-1.0, 1.0] -> [-32768, 32767]
func Float32ToInt16(floats []float32) []int16 {
	pcm := make([]int16, len(floats))
	for i, v := range floats {
		// 限制范围
		if v > 1.0 {
			v = 1.0
		} else if v < -1.0 {
			v = -1.0
		}
		pcm[i] = int16(v * 32767.0)
	}
	return pcm
}

// 计算方法
func calculateERLE(input, output []float32) float64 {
	inputEnergy := calculateRMS(input)
	outputEnergy := calculateRMS(output)

	erleDb := 10 * math.Log10(float64(inputEnergy/outputEnergy))
	return erleDb
}
