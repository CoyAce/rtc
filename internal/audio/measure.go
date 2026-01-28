package audio

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"time"
)

// ==================== 接口定义 ====================
type AudioProvider interface {
	PlayBuffer(data []float32) error
	StartCapture(callback func(data []float32)) error
	StopCapture() error
	GetSampleRate() int
	GetBufferSize() int
	IsPlaying() bool
	IsCapturing() bool
}

// ==================== 延迟测量结果 ====================
type LatencyResult struct {
	Timestamp        time.Time     `json:"timestamp"`
	RoundTripLatency time.Duration `json:"roundtrip_latency_ms"`
	Confidence       float64       `json:"confidence"`
	SampleCount      int           `json:"sample_count"`
}

// ==================== 延迟测量器 ====================
type LatencyMeasurer struct {
	audioProvider AudioProvider
	mu            sync.RWMutex

	// 测量状态
	isMeasuring   bool
	currentResult LatencyResult
	measurements  []time.Duration
	lastPlayback  time.Time

	// 信号处理
	testSignal []float32

	// 回调
	onResult func(LatencyResult)
	onError  func(error)

	// 配置
	sampleRate  int
	bufferSize  int
	maxAttempts int
	timeout     time.Duration
}

// ==================== 配置结构 ====================
type Config struct {
	MaxAttempts        int
	MeasurementTimeout time.Duration
	SignalFrequency    int           // Hz
	SignalDuration     time.Duration // 秒
	PlayVolume         float32
}

var DefaultConfig = Config{
	MaxAttempts:        5,
	MeasurementTimeout: 10 * time.Second,
	SignalFrequency:    1000,
	SignalDuration:     100 * time.Millisecond,
	PlayVolume:         0.3,
}

// ==================== 工厂函数 ====================
func NewLatencyMeasurer(provider AudioProvider, config ...Config) *LatencyMeasurer {
	cfg := DefaultConfig
	if len(config) > 0 {
		cfg = config[0]
	}

	sampleRate := provider.GetSampleRate()
	bufferSize := provider.GetBufferSize()

	return &LatencyMeasurer{
		audioProvider: provider,
		sampleRate:    sampleRate,
		bufferSize:    bufferSize,
		maxAttempts:   cfg.MaxAttempts,
		timeout:       cfg.MeasurementTimeout,
		measurements:  make([]time.Duration, 0, cfg.MaxAttempts),
		testSignal:    generateTestSignal(sampleRate, cfg.SignalFrequency, cfg.SignalDuration, cfg.PlayVolume),
	}
}

// ==================== 公开接口 ====================
// Measure 执行完整的延迟测量
func (m *LatencyMeasurer) Measure(ctx context.Context) (LatencyResult, error) {
	// 清空历史数据
	m.measurements = m.measurements[:0]

	// 测量往返延迟
	roundTripLatency, confidence, err := m.measureRoundTripLatency(ctx)
	if err != nil {
		return LatencyResult{}, fmt.Errorf("round-trip measurement failed: %v", err)
	}
	// 创建结果
	result := LatencyResult{
		Timestamp:        time.Now(),
		RoundTripLatency: roundTripLatency,
		Confidence:       confidence,
		SampleCount:      len(m.measurements),
	}

	m.notifyResult(result)

	return result, nil
}

// GetResult 获取最后一次测量结果
func (m *LatencyMeasurer) GetResult() LatencyResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentResult
}

// IsMeasuring 检查是否正在测量
func (m *LatencyMeasurer) IsMeasuring() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isMeasuring
}

func (m *LatencyMeasurer) measureRoundTripLatency(ctx context.Context) (time.Duration, float64, error) {
	fmt.Println("开始音频回环延迟测量...")

	var results []time.Duration
	// 创建同步通道
	measurementChan := make(chan time.Duration)

	err := m.audioProvider.StartCapture(func(data []float32) {
		// 检查是否包含测试信号
		if len(data) > 0 && m.detectSignal(data) {
			select {
			case measurementChan <- time.Since(m.lastPlayback):
			default:
			}
		}
	})

	if err != nil {
		fmt.Errorf("failed to start capture: %v", err)
	}

	for i := 0; i < m.maxAttempts; i++ {
		// 短暂暂停
		time.Sleep(1000 * time.Millisecond)
		fmt.Printf("测量尝试 %d/%d\n", i+1, m.maxAttempts)

		latency, err := m.singleRoundTripMeasurement(ctx, measurementChan)
		if err != nil {
			fmt.Printf("尝试 %d 失败: %v\n", i+1, err)
			continue
		}
		log.Printf("latency measured: %v\n", latency)

		results = append(results, latency)
		m.measurements = append(m.measurements, latency)

		// 检查上下文
		select {
		case <-ctx.Done():
			return 0, 0, ctx.Err()
		default:
		}
	}

	if len(results) == 0 {
		return 0, 0, fmt.Errorf("no valid measurements obtained")
	}

	// 计算统计信息
	avgLatency := calculateMedian(results)
	confidence := calculateConfidence(results)

	fmt.Printf("回环延迟测量完成: %.2fms (置信度: %.2f, 样本数: %d)\n",
		avgLatency.Seconds()*1000, confidence, len(results))

	return avgLatency, confidence, nil
}

func (m *LatencyMeasurer) singleRoundTripMeasurement(ctx context.Context, measurementChan chan time.Duration) (time.Duration, error) {

	// 记录播放时间并播放测试信号
	m.lastPlayback = time.Now()
	if err := m.audioProvider.PlayBuffer(m.testSignal); err != nil {
		fmt.Printf("failed to play test signal: %v", err)
	}

	// 等待测量结果
	select {
	case latency := <-measurementChan:
		if latency <= 0 {
			return 0, fmt.Errorf("invalid latency measurement")
		}
		return latency, nil

	case <-time.After(m.timeout):
		return 0, fmt.Errorf("measurement timeout")

	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// ==================== 信号处理 ====================
func (m *LatencyMeasurer) detectSignal(data []float32) bool {
	if len(data) == 0 || len(m.testSignal) == 0 {
		return false
	}

	// 使用简单的能量检测
	var energy float64
	for _, sample := range data {
		energy += float64(sample * sample)
	}
	energy /= float64(len(data))

	// 检测阈值
	threshold := 0.001
	return energy > threshold
}

func generateTestSignal(sampleRate int, frequency int, duration time.Duration, volume float32) []float32 {
	samples := int(float64(sampleRate) * duration.Seconds())
	signal := make([]float32, samples)

	// 生成正弦波
	for i := 0; i < samples; i++ {
		t := float64(i) / float64(sampleRate)
		signal[i] = float32(math.Sin(2*math.Pi*float64(frequency)*t)) * volume
	}

	// 添加淡入淡出以避免爆音
	fadeSamples := int(float64(sampleRate) * 0.01) // 10ms淡入淡出
	if fadeSamples > samples/2 {
		fadeSamples = samples / 2
	}

	// 淡入
	for i := 0; i < fadeSamples; i++ {
		fade := float32(i) / float32(fadeSamples)
		signal[i] *= fade
	}

	// 淡出
	for i := 0; i < fadeSamples; i++ {
		fade := float32(fadeSamples-i) / float32(fadeSamples)
		signal[samples-1-i] *= fade
	}

	return signal
}

// ==================== 计算函数 ====================
func (m *LatencyMeasurer) calculateTotalLatency(output, input, processing, roundTrip time.Duration) time.Duration {
	// 使用加权平均计算总延迟
	// 优先使用实际测量的往返延迟
	if roundTrip > 0 {
		// 往返延迟包含了输出+输入+处理延迟
		// 加上一些余量
		return roundTrip
	}

	// 否则使用分量之和
	total := output + input + processing

	// 确保最小值
	if total < 10*time.Millisecond {
		total = 10 * time.Millisecond
	}

	return total
}

func calculateMedian(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	// 排序并取中位数
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	for _, v := range sorted {
		log.Printf("delay: %v", v)
	}

	return sorted[len(sorted)/2]
}

func calculateConfidence(durations []time.Duration) float64 {
	if len(durations) < 2 {
		return 0.5
	}

	// 计算变异系数（CV）
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	mean := float64(sum) / float64(len(durations))

	var variance float64
	for _, d := range durations {
		diff := float64(d) - mean
		variance += diff * diff
	}
	variance /= float64(len(durations))
	stdDev := math.Sqrt(variance)

	// 计算置信度
	cv := stdDev / mean
	confidence := 1.0 - cv

	// 限制范围
	if confidence < 0.1 {
		confidence = 0.1
	}
	if confidence > 0.95 {
		confidence = 0.95
	}

	return confidence
}

// ==================== 回调管理 ====================
func (m *LatencyMeasurer) SetResultCallback(callback func(LatencyResult)) {
	m.onResult = callback
}

func (m *LatencyMeasurer) SetErrorCallback(callback func(error)) {
	m.onError = callback
}

func (m *LatencyMeasurer) notifyResult(result LatencyResult) {
	if m.onResult != nil {
		m.onResult(result)
	}
}

func (m *LatencyMeasurer) notifyError(err error) {
	if m.onError != nil {
		m.onError(err)
	}
}
