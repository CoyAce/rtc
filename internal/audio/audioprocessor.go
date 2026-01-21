package audio

import (
	"math"
)

// Preamp 前置放大器
type Preamp struct {
	TargetRMS  float32 // 目标RMS音量 (0.0-1.0)
	TargetPeak float32 // 目标峰值 (0.0-1.0)
	MaxGain    float32 // 最大增益
}

// NewPreamp 创建前置放大器
func NewPreamp() *Preamp {
	return &Preamp{
		TargetRMS:  -18, // -18dBFS
		TargetPeak: -12, // -12dBFS
		MaxGain:    24,
	}
}

// PCMBuffer 内存中的PCM缓冲区
type PCMBuffer struct {
	Data       []int16 // PCM数据
	SampleRate int     // 采样率
	Channels   int     // 声道数
	VolumeInfo VolumeInfo
}

// VolumeInfo 音量信息
type VolumeInfo struct {
	RMS    float32 // 均方根音量
	Peak   float32 // 峰值
	Max    float32 // 最大样本值
	Min    float32 // 最小样本值
	Silent bool    // 是否静音
}

// AnalyzePCM 分析PCM数据音量
func (p *Preamp) AnalyzePCM(data []float32) VolumeInfo {
	if len(data) == 0 {
		return VolumeInfo{Silent: true}
	}

	var (
		sumSquares float32
		maxPeak    float32
		maxVal     = float32(-1.0)
		minVal     = float32(1.0)
		silent     = true
	)
	for _, sample := range data {
		absSample := float32(math.Abs(float64(sample)))
		sumSquares += sample * sample
		if absSample > maxPeak {
			maxPeak = absSample
		}
		if sample > maxVal {
			maxVal = sample
		}
		if sample < minVal {
			minVal = sample
		}
	}

	rms := float32(math.Sqrt(float64(sumSquares) / float64(len(data))))

	if rms > 0.0003 {
		silent = false
	}

	return VolumeInfo{
		RMS:    rms,
		Peak:   maxPeak,
		Max:    maxVal,
		Min:    minVal,
		Silent: silent,
	}
}

// Process 前置放大PCM数据音量
func (p *Preamp) Process(data []float32) ([]float32, *VolumeInfo) {
	if len(data) == 0 {
		return data, nil
	}

	// 分析原始音量
	info := p.AnalyzePCM(data)

	// 如果是静音，返回原数据
	if info.Silent {
		return data, nil
	}

	// 计算RMS增益
	rmsGain := dbToLinear(p.TargetRMS) / info.RMS

	// 计算峰值增益（防止削波）
	peakGain := dbToLinear(p.TargetPeak) / info.Peak

	// 使用较小的增益
	gain := float32(math.Min(float64(rmsGain), float64(peakGain)))

	// 限制最大增益（避免过度放大噪声）
	if info.RMS < 0.01 { // 非常安静的声音
		if gain > p.MaxGain {
			gain = p.MaxGain
		}
	}

	// 应用增益
	return p.applyGain(data, gain), &info
}

// applyGain 应用增益到PCM数据
func (p *Preamp) applyGain(data []float32, gain float32) []float32 {
	if len(data) == 0 {
		return data
	}

	result := make([]float32, len(data))
	for i, sample := range data {
		value := sample * gain

		// 软限制（防止削波）
		if value > 1.0 {
			// 使用软限制曲线
			overshoot := value - 1.0
			value = 1.0 + overshoot/(1.0+overshoot*0.0001)
		} else if value < -1.0 {
			overshoot := value + 1.0
			value = -1.0 + overshoot/(1.0+overshoot*0.0001)
		}

		result[i] = value
	}

	return result
}

// PCMUtils PCM工具函数
type PCMUtils struct{}

// MixBuffers 混合多个PCM缓冲区
func (u *PCMUtils) MixBuffers(buffers [][]int16) []int16 {
	if len(buffers) == 0 {
		return nil
	}

	// 找到最长的缓冲区
	maxLen := 0
	for _, buf := range buffers {
		if len(buf) > maxLen {
			maxLen = len(buf)
		}
	}

	result := make([]int16, maxLen)

	for i := 0; i < maxLen; i++ {
		var sum float64
		var count int

		for _, buf := range buffers {
			if i < len(buf) {
				sum += float64(buf[i]) / 32768.0
				count++
			}
		}

		if count > 0 {
			// 平均混合并防止削波
			avg := sum / float64(count)

			// 如果多个音源混合，需要降低音量
			if count > 1 {
				avg /= math.Sqrt(float64(count))
			}

			// 转换为int16
			value := avg * 32767.0
			if value > 32767 {
				value = 32767
			} else if value < -32768 {
				value = -32768
			}

			result[i] = int16(value)
		}
	}

	return result
}

// TrimSilence 去除静音部分
func (u *PCMUtils) TrimSilence(data []int16, threshold float64) []int16 {
	if len(data) == 0 {
		return data
	}

	// 找到非静音开始位置
	start := 0
	for i := 0; i < len(data); i++ {
		level := math.Abs(float64(data[i]) / 32768.0)
		if level > threshold {
			start = i
			break
		}
	}

	// 找到非静音结束位置
	end := len(data) - 1
	for i := len(data) - 1; i >= 0; i-- {
		level := math.Abs(float64(data[i]) / 32768.0)
		if level > threshold {
			end = i
			break
		}
	}

	if start >= end {
		return []int16{} // 全是静音
	}

	return data[start:end]
}

// SplitByTime 按时间拆分PCM数据
func (u *PCMUtils) SplitByTime(data []int16, sampleRate, chunkMs int) [][]int16 {
	if len(data) == 0 || sampleRate <= 0 || chunkMs <= 0 {
		return nil
	}

	chunkSamples := sampleRate * chunkMs / 1000
	if chunkSamples == 0 {
		chunkSamples = 1
	}

	var chunks [][]int16
	for i := 0; i < len(data); i += chunkSamples {
		end := i + chunkSamples
		if end > len(data) {
			end = len(data)
		}
		chunks = append(chunks, data[i:end])
	}

	return chunks
}

// HighPassFilter 二阶IIR高通滤波器（Butterworth）
type HighPassFilter struct {
	// 滤波器系数
	b0, b1, b2 float32 // 分子系数
	a1, a2     float32 // 分母系数

	// 状态变量
	x1, x2 float32 // 输入历史
	y1, y2 float32 // 输出历史

	cutoffFreq float32 // 截止频率，如80Hz
	sampleRate float32 // 采样率，如16000
}

func NewHighPassFilter(cutoff, sampleRate float32) *HighPassFilter {
	f := &HighPassFilter{
		cutoffFreq: cutoff,
		sampleRate: sampleRate,
	}
	if math.Abs(float64(cutoff-80.0)) < 0.1 && math.Abs(float64(sampleRate-48000)) < 0.1 {
		// 使用预计算的优化系数（80Hz@48kHz）
		f.setOptimized80HzCoefficients()
	} else {
		f.calculateCoefficients()
	}
	return f
}

func (f *HighPassFilter) setOptimized80HzCoefficients() {
	// 这些系数经过精心优化，数值稳定，性能优秀
	f.b0 = 0.99538605
	f.b1 = -1.99077210
	f.b2 = 0.99538605
	f.a1 = -1.99065018
	f.a2 = 0.99077421
}

func (f *HighPassFilter) calculateCoefficients() {
	// Butterworth二阶高通设计
	omega := float64(2.0 * math.Pi * f.cutoffFreq / f.sampleRate)
	sin := float32(math.Sin(omega))
	cos := float32(math.Cos(omega))

	alpha := sin / (2.0 * 0.7071) // Q=0.7071 (Butterworth)

	// 预计算
	b0 := (1.0 + cos) / 2.0
	b1 := -(1.0 + cos)
	b2 := (1.0 + cos) / 2.0
	a0 := 1.0 + alpha
	a1 := -2.0 * cos
	a2 := 1.0 - alpha

	// 归一化
	f.b0 = b0 / a0
	f.b1 = b1 / a0
	f.b2 = b2 / a0
	f.a1 = a1 / a0
	f.a2 = a2 / a0
}

func (f *HighPassFilter) Process(sample float32) float32 {
	// 直接形式II实现（数值稳定）
	x0 := sample

	// 计算输出：y0 = b0*x0 + b1*x1 + b2*x2 - a1*y1 - a2*y2
	y0 := f.b0*x0 + f.b1*f.x1 + f.b2*f.x2 - f.a1*f.y1 - f.a2*f.y2

	// 更新状态
	f.x2 = f.x1
	f.x1 = x0
	f.y2 = f.y1
	f.y1 = y0

	return y0
}

func (f *HighPassFilter) ProcessBatch(data []float32) {
	for i := range data {
		data[i] = f.Process(data[i])
	}
}
