package audio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gen2brain/malgo"
)

// MiniAudioWrapper 封装miniaudio的音频提供者
type MiniAudioWrapper struct {
	ctx    *malgo.AllocatedContext
	device *malgo.Device

	StreamConfig
	// 播放相关
	playCancel context.CancelFunc
	chunk      chan *bytes.Buffer

	// 采集相关
	captureCancel context.CancelFunc

	// 状态
	isInitialized bool
	isPlaying     bool
	isCapturing   bool
	mu            sync.RWMutex

	// 回调
	captureCallback func([]float32)

	// 配置
	sampleRate int
	bufferSize int
	channels   int
}

// NewMiniAudioWrapper 创建miniaudio封装
func NewMiniAudioWrapper(sampleRate, bufferSize int) (*MiniAudioWrapper, error) {
	wrapper := &MiniAudioWrapper{
		sampleRate: sampleRate,
		bufferSize: bufferSize,
		channels:   1, // 单声道用于延迟测量
	}

	if err := wrapper.initialize(); err != nil {
		return nil, err
	}

	return wrapper, nil
}

func (m *MiniAudioWrapper) initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isInitialized {
		return nil
	}

	// 初始化上下文
	maCtx, _ := malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {
		log.Print("internal/audio: ", message)
	})

	m.ctx = maCtx
	m.isInitialized = true
	config := NewStreamConfig(maCtx, 1)
	config.Format = malgo.FormatF32
	m.StreamConfig = config
	ctx, cancel := context.WithCancel(context.Background())
	m.playCancel = cancel
	pcmChunks := make(chan *bytes.Buffer, 10)
	m.chunk = pcmChunks
	r := NewChunkReader(ctx, pcmChunks)
	go func() {
		if err := Playback(ctx, r, m.StreamConfig); err != nil && !errors.Is(err, io.EOF) {
			log.Printf("audio playback: %v", err)
		}
	}()
	return nil
}

// ==================== AudioProvider接口实现 ====================
func (m *MiniAudioWrapper) PlayBuffer(data []float32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isInitialized {
		return fmt.Errorf("audio wrapper not initialized")
	}
	// 创建播放设备
	r := bytes.NewBuffer(ToBytes(data))
	m.chunk <- r

	m.isPlaying = true
	return nil
}

func (m *MiniAudioWrapper) StartCapture(callback func([]float32)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isInitialized {
		return fmt.Errorf("audio wrapper not initialized")
	}

	if m.isCapturing {
		return fmt.Errorf("already capturing")
	}

	m.captureCallback = callback

	audioChunks := make(chan *bytes.Buffer, 10)
	captureCtx, captureCancel := context.WithCancel(context.Background())
	m.captureCancel = captureCancel
	writer := NewChunkWriter(captureCtx, audioChunks)
	go func() {
		streamConfig := m.StreamConfig
		streamConfig.PeriodSizeInFrames = 120
		streamConfig.Periods = 2
		if err := Capture(captureCtx, writer, streamConfig); err != nil {
			close(audioChunks)
			if errors.Is(err, context.Canceled) {
				return
			}
			log.Printf("capture audio failed, %s", err)
			captureCancel()
		}
	}()
	go func() {
		for buf := range audioChunks {
			if buf != nil && m.isCapturing {
				m.captureCallback(ToFloat32(buf.Bytes()))
			}
		}
	}()
	m.isCapturing = true

	return nil
}

func (m *MiniAudioWrapper) StopCapture() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isCapturing || m.captureCancel == nil {
		return nil
	}

	m.captureCancel()
	m.isCapturing = false
	m.captureCallback = nil

	return nil
}

func (m *MiniAudioWrapper) GetSampleRate() int {
	return m.sampleRate
}

func (m *MiniAudioWrapper) GetBufferSize() int {
	return m.bufferSize
}

func (m *MiniAudioWrapper) IsPlaying() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isPlaying
}

func (m *MiniAudioWrapper) IsCapturing() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isCapturing
}

// ==================== 清理 ====================
func (m *MiniAudioWrapper) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isPlaying && m.playCancel != nil {
		m.playCancel()
	}

	if m.isCapturing && m.captureCancel != nil {
		m.captureCancel()
	}

	if m.ctx != nil {
		m.ctx.Uninit()
		m.ctx = nil
	}

	m.isInitialized = false
	m.isPlaying = false
	m.isCapturing = false

	return nil
}

func TestAudioLatency() {
	fmt.Println("=== macOS音频延迟测量工具 ===")

	// 初始化音频系统
	w, err := NewMiniAudioWrapper(48000, 960)
	if err != nil {
		fmt.Printf("音频初始化失败: %v\n", err)
		return
	}
	defer w.Close()

	// 创建延迟测量器
	conf := Config{
		MaxAttempts:        20,
		MeasurementTimeout: 1 * time.Second,
		SignalFrequency:    1000,
		SignalDuration:     200 * time.Millisecond,
		PlayVolume:         0.2,
	}

	measurer := NewLatencyMeasurer(w, conf)

	// 设置回调
	measurer.SetResultCallback(func(result LatencyResult) {
		fmt.Println("\n", formatResult(result))
	})

	measurer.SetErrorCallback(func(err error) {
		fmt.Printf("测量错误: %v\n", err)
	})

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 创建测量上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 执行测量
	go func() {
		fmt.Println("\n开始测量音频系统延迟...")

		result, err := measurer.Measure(ctx)
		if err != nil {
			fmt.Printf("测量失败: %v\n", err)
			return
		}
		fmt.Println("\n", formatResult(result))

	}()

	// 等待退出
	<-sigChan
}

func formatResult(result LatencyResult) string {
	return fmt.Sprintf(`=== 音频延迟测量结果 ===
测量时间: %s
  往返延迟: %.2fms
置信度: %.2f
测量样本数: %d
=========================`,
		result.Timestamp.Format("15:04:05"),
		result.RoundTripLatency.Seconds()*1000,
		result.Confidence,
		result.SampleCount)
}
