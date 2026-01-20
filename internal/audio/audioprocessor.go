package audio

import (
	"math"
)

// Preamp 前置放大器
type Preamp struct {
	TargetRMS  float64 // 目标RMS音量 (0.0-1.0)
	TargetPeak float64 // 目标峰值 (0.0-1.0)
	MaxGain    float64 // 最大增益
}

// NewPreamp 创建前置放大器
func NewPreamp() *Preamp {
	return &Preamp{
		TargetRMS:  -24, // -24dBFS
		TargetPeak: -18, // -18dBFS
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
	RMS    float64 // 均方根音量
	Peak   float64 // 峰值
	Max    float64 // 最大样本值
	Min    float64 // 最小样本值
	Silent bool    // 是否静音
}

// AnalyzePCM 分析PCM数据音量
func (p *Preamp) AnalyzePCM(data []float64) VolumeInfo {
	if len(data) == 0 {
		return VolumeInfo{Silent: true}
	}

	var (
		sumSquares float64
		maxPeak    float64
		maxVal     = -1.0
		minVal     = 1.0
		silent     = true
	)
	for _, sample := range data {
		absSample := math.Abs(sample)
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

	rms := math.Sqrt(sumSquares / float64(len(data)))

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
func (p *Preamp) Process(data []float64) ([]float64, *VolumeInfo) {
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
	gain := math.Min(rmsGain, peakGain)

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
func (p *Preamp) applyGain(data []float64, gain float64) []float64 {
	if len(data) == 0 {
		return data
	}

	result := make([]float64, len(data))
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
