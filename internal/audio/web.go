package audio

import (
	"math"
	"sync"
)

// EchoMetrics 回声消除性能指标
type EchoMetrics struct {
	ERLE         float32 // 回声返回损耗增强 (dB)
	ResidualEcho float32 // 残留回声水平
	Convergence  float32 // 收敛度 (0-1)
}

// NLMSAEC 基于NLMS（归一化最小均方）算法的回声消除器
type NLMSAEC struct {
	// 滤波器参数
	filterLength   int       // 滤波器长度（建议：64-256ms的回声尾长度）
	filterCoeffs   []float32 // 滤波器系数
	filterHistory  []float32 // 参考信号历史
	stepSize       float32   // 步长 (0.1-0.5)
	regularization float32   // 正则化参数 (1e-6-1e-9)

	// 信号参数
	sampleRate int
	channels   int
	frameSize  int // 20ms帧大小 = 960样本

	// 状态变量
	mu          sync.RWMutex
	isAdapting  bool
	convergence float32
	frameCount  int

	// 双讲检测
	doubleTalkThreshold float32
	doubleTalkDetected  bool

	// 残留回声抑制
	residualEchoSuppressor *ResidualEchoSuppressor
}

// NewNLMSAEC 创建新的NLMS回声消除器
func NewNLMSAEC(sampleRate, channels int) *NLMSAEC {
	frameDuration := 0.020 // 20ms
	frameSize := int(float64(sampleRate) * frameDuration)

	// 滤波器长度：假设最大回声延迟128ms
	filterLength := int(float64(sampleRate) * 0.128) // 6144个样本

	aec := &NLMSAEC{
		sampleRate:          sampleRate,
		channels:            channels,
		frameSize:           frameSize,
		filterLength:        filterLength,
		filterCoeffs:        make([]float32, filterLength),
		filterHistory:       make([]float32, filterLength),
		stepSize:            0.3,
		regularization:      1e-7,
		isAdapting:          true,
		convergence:         0.0,
		doubleTalkThreshold: 0.1,
	}

	// 初始化残留回声抑制器
	aec.residualEchoSuppressor = NewResidualEchoSuppressor(sampleRate, channels)

	return aec
}

// Process 处理一帧音频数据
func (aec *NLMSAEC) Process(reference, mic []float32) []float32 {
	aec.mu.Lock()
	defer aec.mu.Unlock()

	return aec.processChannel(reference, mic)
}

// processChannel 处理单声道
func (aec *NLMSAEC) processChannel(reference, mic []float32) []float32 {
	frameSize := len(reference)
	output := make([]float32, frameSize)

	// 更新参考信号历史
	if reference != nil {
		aec.updateReferenceHistory(reference)
	}

	for n := 0; n < frameSize; n++ {
		// 构建当前输入向量
		inputVector := aec.getInputVector(n + frameSize - 1)

		// 计算回声估计
		echoEstimate := float32(0.0)
		for i := 0; i < aec.filterLength; i++ {
			echoEstimate += aec.filterCoeffs[i] * inputVector[i]
		}

		// 计算误差（麦克风信号 - 回声估计）
		error := mic[n] - echoEstimate

		// 双讲检测
		isDoubleTalk := aec.detectDoubleTalk(mic[n], echoEstimate)

		// NLMS滤波器更新（无双讲时更新）
		if aec.isAdapting && !isDoubleTalk {
			aec.updateFilterCoeffs(inputVector, error)
		}

		// 应用残留回声抑制
		output[n] = aec.residualEchoSuppressor.ProcessSample(error, echoEstimate)
	}

	aec.frameCount++
	return output
}

// updateReferenceHistory 更新参考信号历史
func (aec *NLMSAEC) updateReferenceHistory(reference []float32) {
	// 将新帧添加到历史缓冲区
	copy(aec.filterHistory, aec.filterHistory[len(reference):])
	copy(aec.filterHistory[aec.filterLength-len(reference):], reference)
}

// getInputVector 获取输入向量
func (aec *NLMSAEC) getInputVector(offset int) []float32 {
	vector := make([]float32, aec.filterLength)
	for i := 0; i < aec.filterLength; i++ {
		idx := offset - i
		if idx >= 0 && idx < aec.filterLength {
			vector[i] = aec.filterHistory[idx]
		}
	}
	return vector
}

// updateFilterCoeffs 更新滤波器系数（NLMS算法）
func (aec *NLMSAEC) updateFilterCoeffs(inputVector []float32, error float32) {
	// 计算输入向量的功率
	inputPower := float32(0.0)
	for i := 0; i < aec.filterLength; i++ {
		inputPower += inputVector[i] * inputVector[i]
	}

	// 归一化步长
	normalizedStep := aec.stepSize / (inputPower + aec.regularization)

	// 更新滤波器系数
	for i := 0; i < aec.filterLength; i++ {
		aec.filterCoeffs[i] += normalizedStep * error * inputVector[i]
	}

	// 更新收敛度估计
	aec.updateConvergence(error, inputPower)
}

// detectDoubleTalk 双讲检测
func (aec *NLMSAEC) detectDoubleTalk(micSample, echoEstimate float32) bool {
	micPower := micSample * micSample
	echoPower := echoEstimate * echoEstimate

	// 简单的双讲检测：如果麦克风功率远大于回声估计功率
	ratio := micPower / (echoPower + 1e-10)
	isDoubleTalk := ratio > aec.doubleTalkThreshold

	// 更新状态
	if isDoubleTalk != aec.doubleTalkDetected {
		aec.doubleTalkDetected = isDoubleTalk
		if isDoubleTalk {
			// 双讲时减少步长
			aec.stepSize = 0.1
		} else {
			// 恢复步长
			aec.stepSize = 0.3
		}
	}

	return isDoubleTalk
}

// updateConvergence 更新收敛度估计
func (aec *NLMSAEC) updateConvergence(error, inputPower float32) {
	errorPower := error * error
	erle := 10 * math.Log10(float64((inputPower+1e-10)/(errorPower+1e-10)))

	// 平滑收敛度估计
	alpha := float32(0.95)
	aec.convergence = alpha*aec.convergence + (1-alpha)*float32(math.Min(1.0, erle/30.0))
}

// GetMetrics 获取性能指标
func (aec *NLMSAEC) GetMetrics() EchoMetrics {
	aec.mu.RLock()
	defer aec.mu.RUnlock()

	return EchoMetrics{
		ERLE:         20 * float32(math.Log10(float64(1/(1-aec.convergence+1e-10)))),
		ResidualEcho: aec.residualEchoSuppressor.GetResidualLevel(),
		Convergence:  aec.convergence,
	}
}

// Reset 重置回声消除器
func (aec *NLMSAEC) Reset() {
	aec.mu.Lock()
	defer aec.mu.Unlock()

	// 重置滤波器
	for i := range aec.filterCoeffs {
		aec.filterCoeffs[i] = 0.0
	}
	for i := range aec.filterHistory {
		aec.filterHistory[i] = 0.0
	}

	// 重置状态
	aec.isAdapting = true
	aec.convergence = 0.0
	aec.frameCount = 0
	aec.doubleTalkDetected = false
	aec.stepSize = 0.3

	// 重置残留回声抑制器
	aec.residualEchoSuppressor.Reset()
}

// ResidualEchoSuppressor 残留回声抑制器
type ResidualEchoSuppressor struct {
	sampleRate      int
	channels        int
	frameSize       int
	noiseEstimator  *NoiseEstimator
	echoEstimator   *EchoEstimator
	suppressionGain float32
	residualLevel   float32
}

// NewResidualEchoSuppressor 创建残留回声抑制器
func NewResidualEchoSuppressor(sampleRate, channels int) *ResidualEchoSuppressor {
	frameSize := int(float64(sampleRate) * 0.020)

	return &ResidualEchoSuppressor{
		sampleRate:      sampleRate,
		channels:        channels,
		frameSize:       frameSize,
		noiseEstimator:  NewNoiseEstimator(sampleRate),
		echoEstimator:   NewEchoEstimator(sampleRate),
		suppressionGain: 0.1,
	}
}

// ProcessSample 处理单个样本
func (res *ResidualEchoSuppressor) ProcessSample(error, echoEstimate float32) float32 {
	// 估计噪声和残留回声
	noiseLevel := res.noiseEstimator.Estimate(error)
	residualEcho := res.echoEstimator.Estimate(echoEstimate)

	// 计算后置滤波增益
	signalPower := error*error + 1e-10
	echoPower := residualEcho*residualEcho + 1e-10
	noisePower := noiseLevel*noiseLevel + 1e-10

	// 计算先验SNR和后验SNR
	posteriorSNR := signalPower / (echoPower + noisePower)
	priorSNR := 0.98*float32(res.echoEstimator.lastPriorSNR) + 0.02*float32(math.Max(0, float64(posteriorSNR-1)))
	res.echoEstimator.lastPriorSNR = priorSNR

	// 计算增益（MMSE-STSA增益）
	gain := priorSNR / (1 + priorSNR)

	// 应用抑制
	suppressed := error * float32(math.Sqrt(float64(gain)))

	// 更新残留回声水平
	res.residualLevel = 0.95*res.residualLevel + 0.05*float32(math.Sqrt(float64(echoPower)))

	return suppressed
}

// GetResidualLevel 获取残留回声水平
func (res *ResidualEchoSuppressor) GetResidualLevel() float32 {
	return res.residualLevel
}

// Reset 重置抑制器
func (res *ResidualEchoSuppressor) Reset() {
	res.noiseEstimator.Reset()
	res.echoEstimator.Reset()
	res.residualLevel = 0.0
}

// NoiseEstimator 噪声估计器
type NoiseEstimator struct {
	noiseLevel float32
	minNoise   float32
	smoothing  float32
	frameCount int
}

// NewNoiseEstimator 创建噪声估计器
func NewNoiseEstimator(sampleRate int) *NoiseEstimator {
	return &NoiseEstimator{
		noiseLevel: 1e-4,
		minNoise:   1e-6,
		smoothing:  0.98,
	}
}

// Estimate 估计噪声水平
func (ne *NoiseEstimator) Estimate(signal float32) float32 {
	signalPower := signal * signal

	// 使用最小值统计法更新噪声估计
	if ne.frameCount < 100 {
		// 初始阶段直接平均
		ne.noiseLevel = (ne.noiseLevel*float32(ne.frameCount) + signalPower) / float32(ne.frameCount+1)
	} else {
		// 平滑更新
		alpha := ne.smoothing
		if signalPower < ne.noiseLevel {
			alpha = 0.99 // 噪声降低时慢速更新
		}
		ne.noiseLevel = alpha*ne.noiseLevel + (1-alpha)*signalPower
	}

	ne.frameCount++
	ne.noiseLevel = float32(math.Max(float64(ne.noiseLevel), float64(ne.minNoise)))

	return float32(math.Sqrt(float64(ne.noiseLevel)))
}

// Reset 重置噪声估计器
func (ne *NoiseEstimator) Reset() {
	ne.noiseLevel = 1e-4
	ne.frameCount = 0
}

// EchoEstimator 回声估计器
type EchoEstimator struct {
	echoLevel    float32
	lastEcho     float32
	lastPriorSNR float32
	smoothing    float32
}

// NewEchoEstimator 创建回声估计器
func NewEchoEstimator(sampleRate int) *EchoEstimator {
	return &EchoEstimator{
		echoLevel: 1e-4,
		smoothing: 0.9,
	}
}

// Estimate 估计回声水平
func (ee *EchoEstimator) Estimate(echoEstimate float32) float32 {
	echoPower := echoEstimate * echoEstimate

	// 平滑更新回声估计
	alpha := ee.smoothing
	if echoPower < ee.echoLevel {
		alpha = 0.95 // 回声降低时慢速更新
	}
	ee.echoLevel = alpha*ee.echoLevel + (1-alpha)*echoPower
	ee.lastEcho = echoEstimate

	return float32(math.Sqrt(float64(ee.echoLevel)))
}

// Reset 重置回声估计器
func (ee *EchoEstimator) Reset() {
	ee.echoLevel = 1e-4
	ee.lastEcho = 0.0
	ee.lastPriorSNR = 0.0
}

// 工具函数
func int16ToFloat64(data []int16, channels int) [][]float64 {
	frameSize := len(data) / channels
	result := make([][]float64, channels)

	for ch := 0; ch < channels; ch++ {
		result[ch] = make([]float64, frameSize)
		for i := 0; i < frameSize; i++ {
			idx := i*channels + ch
			result[ch][i] = float64(data[idx]) / 32768.0
		}
	}

	return result
}

func float64ToInt16(data [][]float64, channels int) []int16 {
	frameSize := len(data[0])
	result := make([]int16, frameSize*channels)

	for i := 0; i < frameSize; i++ {
		for ch := 0; ch < channels; ch++ {
			idx := i*channels + ch
			val := data[ch][i]

			// 限制在[-1, 1]范围内
			if val > 1.0 {
				val = 1.0
			} else if val < -1.0 {
				val = -1.0
			}

			result[idx] = int16(val * 32767.0)
		}
	}

	return result
}

// WebRTCAEC 基于WebRTC AEC3的增强回声消除器
type WebRTCAEC struct {
	nlmsAEC            *NLMSAEC
	delayEstimator     *DelayEstimator
	nonlinearProcessor *NonlinearProcessor
	config             AECConfig
	metrics            EchoMetrics
}

type AECConfig struct {
	SampleRate                int
	Channels                  int
	FilterLength              int // 滤波器长度（毫秒）
	EnableDelayCorrection     bool
	EnableNonlinearProcessing bool
}

// NewWebRTCAEC 创建增强的回声消除器
func NewWebRTCAEC(config AECConfig) *WebRTCAEC {
	if config.SampleRate == 0 {
		config.SampleRate = 48000
	}
	if config.Channels == 0 {
		config.Channels = 2
	}
	if config.FilterLength == 0 {
		config.FilterLength = 128 // 128ms
	}

	aec := &WebRTCAEC{
		nlmsAEC: NewNLMSAEC(config.SampleRate, config.Channels),
		config:  config,
	}

	if config.EnableNonlinearProcessing {
		aec.nonlinearProcessor = NewNonlinearProcessor(config.SampleRate)
	}

	return aec
}

// Process 处理一帧数据
func (waec *WebRTCAEC) Process(reference, mic []float32) []float32 {
	// 2. NLMS回声消除
	output := waec.nlmsAEC.Process(reference, mic)

	// 3. 非线性处理（抑制残留回声）
	if waec.nonlinearProcessor != nil {
		output = waec.nonlinearProcessor.Process(output, reference)
	}

	// 4. 更新指标
	waec.metrics = waec.nlmsAEC.GetMetrics()

	return output
}

// Reset 重置
func (waec *WebRTCAEC) Reset() {
	waec.nlmsAEC.Reset()
	if waec.delayEstimator != nil {
		waec.delayEstimator.Reset()
	}
	if waec.nonlinearProcessor != nil {
		waec.nonlinearProcessor.Reset()
	}
}

// GetMetrics 获取指标
func (waec *WebRTCAEC) GetMetrics() EchoMetrics {
	return waec.metrics
}

// NonlinearProcessor 非线性处理器（抑制残留回声）
type NonlinearProcessor struct {
	suppressionGain []float64
	noiseEstimator  *NoiseEstimator
	echoEstimator   *EchoEstimator
	sampleRate      int
}

// NewNonlinearProcessor 创建非线性处理器
func NewNonlinearProcessor(sampleRate int) *NonlinearProcessor {
	frameSize := int(float64(sampleRate) * 0.020)

	return &NonlinearProcessor{
		suppressionGain: make([]float64, frameSize),
		noiseEstimator:  NewNoiseEstimator(sampleRate),
		echoEstimator:   NewEchoEstimator(sampleRate),
		sampleRate:      sampleRate,
	}
}

// Process 处理一帧数据
func (nlp *NonlinearProcessor) Process(signal, reference []float32) []float32 {
	frameSize := len(signal)

	output := make([]float32, frameSize)

	for i := 0; i < frameSize; i++ {
		// 估计噪声和回声
		noiseLevel := nlp.noiseEstimator.Estimate(signal[i])
		echoLevel := nlp.echoEstimator.Estimate(reference[i])

		// 计算信号功率
		signalPower := signal[i] * signal[i]
		totalDisturbance := echoLevel*echoLevel + noiseLevel*noiseLevel

		// 计算增益
		snr := signalPower / (totalDisturbance + 1e-10)
		gain := snr / (1 + snr)

		// 应用非线性抑制
		nlp.suppressionGain[i] = 0.9*nlp.suppressionGain[i] + 0.1*float64(gain)
		output[i] = signal[i] * float32(math.Sqrt(nlp.suppressionGain[i]))
	}

	return output
}

func (nlp *NonlinearProcessor) Reset() {
	for i := range nlp.suppressionGain {
		nlp.suppressionGain[i] = 1.0
	}
	nlp.noiseEstimator.Reset()
	nlp.echoEstimator.Reset()
}

// DefaultAEC create default aec
func DefaultAEC() *WebRTCAEC {
	config := AECConfig{
		SampleRate:                48000,
		Channels:                  1,
		FilterLength:              128,
		EnableNonlinearProcessing: true,
	}

	return NewWebRTCAEC(config)
}
