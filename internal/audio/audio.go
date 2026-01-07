package audio

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"runtime"
	"strings"

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
	abortChan := make(chan error)
	defer close(abortChan)
	aborted := false

	deviceCallbacks := malgo.DeviceCallbacks{
		Data: func(outputSamples, inputSamples []byte, frameCount uint32) {
			if aborted {
				return
			}

			if runtime.GOOS == "android" {
				inputSamples = increasePcmBytesVolume(inputSamples, 100)
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
	abortChan := make(chan error)
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

func toPcmInts(pcm []byte) []int16 {
	ret := make([]int16, len(pcm)/2)
	for i := 0; i < len(pcm); i += 2 {
		ret[i/2] = int16(binary.NativeEndian.Uint16(pcm[i : i+2]))
	}
	return ret
}

func toPcmBytes(pcm []int16) []byte {
	ret := make([]byte, len(pcm)*2)
	for i, v := range pcm {
		binary.NativeEndian.PutUint16(ret[i*2:], uint16(v)) // 将int16值写入到byte切片中
	}
	return ret
}

func increasePcmBytesVolume(pcm []byte, volumeFactor float64) []byte {
	return toPcmBytes(increaseVolume(toPcmInts(pcm), volumeFactor))
}

// increaseVolume 增加PCM音频数据的音量
// pcmData 是原始PCM数据，volumeFactor 是音量增加的因子（例如：2.0表示加倍音量）
func increaseVolume(pcm []int16, volumeFactor float64) []int16 {
	var maxValue int16 = 32767  // 16位PCM的最大值
	var minValue int16 = -32768 // 16位PCM的最小值
	var ret []int16

	for _, sample := range pcm {
		// 计算新的样本值
		resample := int16(float64(sample) * volumeFactor)
		// 确保新样本值在有效范围内
		if resample > maxValue {
			resample = maxValue
		} else if resample < minValue {
			resample = minValue
		}
		ret = append(ret, resample)
	}

	return ret
}
