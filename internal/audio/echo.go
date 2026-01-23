package audio

import (
	"math"
	"sync"
	"time"
)

// 常量定义
const (
	SampleRate       = 48000 // 采样率
	FrameMs          = 20    // 帧时长（毫秒）
	FrameSize        = 960   // 帧大小：48000 * 20 / 1000 = 960
	MaxEchoDelayMs   = 500   // 最大回声延迟（毫秒）
	MaxEchoDelay     = 24000 // 最大回声延迟样本数：48000 * 500 / 1000
	FilterLengthMs   = 200   // 滤波器长度（毫秒）
	FilterLength     = 9600  // 滤波器长度样本数：48000 * 200 / 1000
	DoubleTalkWindow = 10    // 双讲检测窗口（帧数）
	ERLECalcWindow   = 50    // ERLE计算窗口（帧数）
)

// Int16ToFloat32 - int16转float32 [-32768, 32767] -> [-1.0, 1.0]
func Int16ToFloat32(pcm []int16) []float32 {
	floats := make([]float32, len(pcm))
	for i, v := range pcm {
		floats[i] = float32(v) / 32768.0
	}
	return floats
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

// ==================== 环形缓冲区 ====================
type CircularBuffer struct {
	data     []float32
	size     int
	capacity int
	head     int
	tail     int
	mu       sync.RWMutex
}

func NewCircularBuffer(capacity int) *CircularBuffer {
	return &CircularBuffer{
		data:     make([]float32, capacity),
		capacity: capacity,
	}
}

func (cb *CircularBuffer) Write(samples []float32) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	for _, s := range samples {
		cb.data[cb.head] = s
		cb.head = (cb.head + 1) % cb.capacity

		if cb.size < cb.capacity {
			cb.size++
		} else {
			cb.tail = (cb.tail + 1) % cb.capacity
		}
	}
}

func (cb *CircularBuffer) Read(size int) []float32 {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if size > cb.size {
		size = cb.size
	}

	result := make([]float32, size)
	for i := 0; i < size; i++ {
		idx := (cb.tail + i) % cb.capacity
		result[i] = cb.data[idx]
	}

	return result
}

func (cb *CircularBuffer) ReadLastN(size int) []float32 {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	result := make([]float32, size)
	for i := 0; i < cb.size && i < size; i++ {
		idx := (cb.head - (size - i) + cb.capacity) % cb.capacity
		result[i] = cb.data[idx]
	}

	return result

}

func (cb *CircularBuffer) Peek(start, size int) []float32 {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	if cb.size == 0 || size > cb.size {
		return nil
	}

	result := make([]float32, size)
	for i := 0; i < size; i++ {
		idx := (start + i) % cb.capacity
		if idx < len(cb.data) {
			result[i] = cb.data[idx]
		}
	}
	return result
}

func (cb *CircularBuffer) Size() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.size
}

func (cb *CircularBuffer) Clear() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.head = 0
	cb.tail = 0
	cb.size = 0
}

// ==================== 延时估计器 ====================
type DelayEstimator struct {
	maxDelay           int
	historySize        int
	farHistory         *CircularBuffer
	nearHistory        *CircularBuffer
	farHistoryUpdateAt time.Time
	correlation        []float32
	smoothing          float32
	currentDelay       int
	updatePeriod       int
	frameCount         int
	minCorrelation     float32
}

func NewDelayEstimator() *DelayEstimator {
	return &DelayEstimator{
		maxDelay:       MaxEchoDelay,
		historySize:    MaxEchoDelay * 2,
		farHistory:     NewCircularBuffer(MaxEchoDelay * 2),
		nearHistory:    NewCircularBuffer(MaxEchoDelay * 2),
		correlation:    make([]float32, MaxEchoDelay),
		smoothing:      0.95,
		currentDelay:   1060, // 初始假设2ms延迟
		updatePeriod:   5,    // 每5帧更新一次
		minCorrelation: 0.3,
	}
}

func (de *DelayEstimator) AdjustDelay(frameSize int) []float32 {
	if time.Now().Sub(de.farHistoryUpdateAt) > time.Duration(500)*time.Millisecond {
		return nil
	}
	return de.farHistory.ReadLastN(frameSize)
}

func (de *DelayEstimator) Estimate(nearEnd []float32) int {
	// 更新历史缓冲区
	de.nearHistory.Write(nearEnd)

	de.frameCount++

	// 定期更新延时估计
	if de.frameCount%de.updatePeriod == 0 && de.farHistory.Size() >= de.historySize {
		de.computeCorrelation()
		de.findBestDelay()
	}

	return de.currentDelay
}

func (de *DelayEstimator) computeCorrelation() {
	// 获取历史数据
	farData := de.farHistory.Read(de.historySize)
	nearData := de.nearHistory.Read(de.historySize)

	if len(farData) != de.historySize || len(nearData) != de.historySize {
		return
	}

	// 计算互相关
	correlationWindow := 2048 // 约42.7ms窗口

	for d := 0; d < de.maxDelay; d++ {
		var corr float32
		var farPower float32

		// 计算相关系数
		for i := 0; i < correlationWindow; i++ {
			farIdx := de.historySize - correlationWindow + i
			nearIdx := farIdx - d

			if nearIdx >= 0 {
				farVal := farData[farIdx]
				nearVal := nearData[nearIdx]

				corr += farVal * nearVal
				farPower += farVal * farVal
			}
		}

		if farPower > 0 {
			de.correlation[d] = corr / float32(math.Sqrt(float64(farPower)))
		} else {
			de.correlation[d] = 0
		}
	}
}

func (de *DelayEstimator) findBestDelay() {
	maxCorr := float32(-1.0)
	bestDelay := de.currentDelay

	// 寻找最大相关系数，排除边界
	searchMin := 50   // 约1ms
	searchMax := 2000 // 约41.7ms

	if searchMax > de.maxDelay {
		searchMax = de.maxDelay
	}

	for d := searchMin; d < searchMax; d++ {
		if de.correlation[d] > maxCorr {
			maxCorr = de.correlation[d]
			bestDelay = d
		}
	}

	// 平滑更新延时估计
	if maxCorr > de.minCorrelation {
		de.currentDelay = int(de.smoothing*float32(de.currentDelay) +
			(1-de.smoothing)*float32(bestDelay))
	}
}

func (de *DelayEstimator) Reset() {
	de.farHistory.Clear()
	de.nearHistory.Clear()
	de.currentDelay = 100
	de.frameCount = 0
}

// 计算方法
func calculateERLE(input, output []float32) float64 {
	inputEnergy := calculateRMS(input)
	outputEnergy := calculateRMS(output)

	erleDb := 10 * math.Log10(float64(inputEnergy/outputEnergy))
	return erleDb
}
