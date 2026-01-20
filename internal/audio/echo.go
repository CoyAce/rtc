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

// Int16ToFloat64 - int16转float32 [-32768, 32767] -> [-1.0, 1.0]
func Int16ToFloat64(pcm []int16) []float64 {
	floats := make([]float64, len(pcm))
	for i, v := range pcm {
		floats[i] = float64(v) / 32768.0
	}
	return floats
}

// Float64ToInt16 - float32转int16 [-1.0, 1.0] -> [-32768, 32767]
func Float64ToInt16(floats []float64) []int16 {
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
	data     []float64
	size     int
	capacity int
	head     int
	tail     int
	mu       sync.RWMutex
}

func NewCircularBuffer(capacity int) *CircularBuffer {
	return &CircularBuffer{
		data:     make([]float64, capacity),
		capacity: capacity,
	}
}

func (cb *CircularBuffer) Write(samples []float64) {
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

func (cb *CircularBuffer) Read(size int) []float64 {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if size > cb.size {
		size = cb.size
	}

	result := make([]float64, size)
	for i := 0; i < size; i++ {
		idx := (cb.tail + i) % cb.capacity
		result[i] = cb.data[idx]
	}

	return result
}

func (cb *CircularBuffer) Peek(start, size int) []float64 {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	result := make([]float64, size)
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
	maxDelay       int
	historySize    int
	farHistory     *CircularBuffer
	nearHistory    *CircularBuffer
	correlation    []float64
	smoothing      float64
	currentDelay   int
	updatePeriod   int
	frameCount     int
	minCorrelation float64
}

func NewDelayEstimator() *DelayEstimator {
	return &DelayEstimator{
		maxDelay:       MaxEchoDelay,
		historySize:    MaxEchoDelay * 2,
		farHistory:     NewCircularBuffer(MaxEchoDelay * 2),
		nearHistory:    NewCircularBuffer(MaxEchoDelay * 2),
		correlation:    make([]float64, MaxEchoDelay),
		smoothing:      0.95,
		currentDelay:   100, // 初始假设2ms延迟
		updatePeriod:   5,   // 每5帧更新一次
		minCorrelation: 0.3,
	}
}

func (de *DelayEstimator) AdjustDelay(reference []float64, delay int) []float64 {
	if delay == 0 {
		return reference
	}

	frameSize := len(reference)
	adjusted := make([]float64, frameSize)

	if delay > 0 {
		// 参考信号延迟，需要提前
		if delay < frameSize {
			copy(adjusted, reference[delay:])
			// 填充剩余部分
			for i := frameSize - delay; i < frameSize; i++ {
				adjusted[i] = 0
			}
		} else {
			// 延迟过大，返回静音
			for i := range adjusted {
				adjusted[i] = 0
			}
		}
	} else {
		// 参考信号超前，需要延迟
		delay = -delay
		if delay < frameSize {
			for i := 0; i < delay; i++ {
				adjusted[i] = 0
			}
			copy(adjusted[delay:], reference[:frameSize-delay])
		} else {
			for i := range adjusted {
				adjusted[i] = 0
			}
		}
	}

	return adjusted
}

func (de *DelayEstimator) Estimate(farEnd, nearEnd []float64) int {
	// 更新历史缓冲区
	de.farHistory.Write(farEnd)
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
		var corr float64
		var farPower float64

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
			de.correlation[d] = corr / float64(math.Sqrt(float64(farPower)))
		} else {
			de.correlation[d] = 0
		}
	}
}

func (de *DelayEstimator) findBestDelay() {
	maxCorr := float64(-1.0)
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
		de.currentDelay = int(de.smoothing*float64(de.currentDelay) +
			(1-de.smoothing)*float64(bestDelay))
	}
}

func (de *DelayEstimator) Reset() {
	de.farHistory.Clear()
	de.nearHistory.Clear()
	de.currentDelay = 100
	de.frameCount = 0
}

func calculatePower(samples []float64) float64 {
	var power float64
	for _, s := range samples {
		power += s * s
	}
	return power / float64(len(samples))
}

// ==================== 回声消除统计 ====================
type EchoStats struct {
	ERLE            float64 // 回声消除比（dB）
	FarPower        float64 // 远端信号功率（dB）
	NearPower       float64 // 近端信号功率（dB）
	ResidualPower   float64 // 残差信号功率（dB）
	Delay           int     // 当前延时估计（样本）
	DoubleTalk      bool    // 是否双讲
	FilterConverged bool    // 滤波器是否收敛
	FrameCount      int64   // 处理的帧数
	StartTime       time.Time
	mu              sync.RWMutex
}

func NewEchoStats() *EchoStats {
	return &EchoStats{
		StartTime: time.Now(),
	}
}

func (es *EchoStats) Update(farEnd, nearEnd, residual []float64, delay int, doubleTalk bool) {
	es.mu.Lock()
	defer es.mu.Unlock()

	// 计算功率
	farPower := calculatePower(farEnd)
	nearPower := calculatePower(nearEnd)
	residualPower := calculatePower(residual)

	// 平滑更新
	alpha := float64(0.1) // 平滑系数
	es.FarPower = (1-alpha)*es.FarPower + alpha*farPower
	es.NearPower = (1-alpha)*es.NearPower + alpha*nearPower
	es.ResidualPower = (1-alpha)*es.ResidualPower + alpha*residualPower

	// 计算ERLE（dB）
	if residualPower > 0 && farPower > 0 {
		erle := 10 * float64(math.Log10(float64(nearPower/residualPower)))
		es.ERLE = (1-alpha)*es.ERLE + alpha*erle
	}

	es.Delay = delay
	es.DoubleTalk = doubleTalk
	es.FrameCount++

	// 检查滤波器收敛
	if es.ERLE > 15 {
		es.FilterConverged = true
	}
}

func (es *EchoStats) GetERLE() float64 {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.ERLE
}

func (es *EchoStats) GetDelay() int {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.Delay
}

func (es *EchoStats) GetUptime() time.Duration {
	return time.Since(es.StartTime)
}

// ==================== 主回声消除器 ====================
type RealTimeEchoCancel struct {
	filter         *NLMSAEC
	delayEstimator *DelayEstimator
	farBuffer      *CircularBuffer
	stats          *EchoStats
	params         *EchoParams
	enabled        bool
	initialized    bool
	initFrames     int
	mu             sync.RWMutex
}

type EchoParams struct {
	Enabled              bool
	FilterLength         int
	Mu                   float64
	InitializationFrames int
}

func DefaultParams() *EchoParams {
	return &EchoParams{
		Enabled:              true,
		FilterLength:         FilterLength,
		Mu:                   0.3,
		InitializationFrames: 50, // 1秒初始化时间
	}
}

func NewRealTimeEchoCancel(params *EchoParams) *RealTimeEchoCancel {
	if params == nil {
		params = DefaultParams()
	}

	return &RealTimeEchoCancel{
		filter:         NewNLMSAEC(SampleRate, 2),
		delayEstimator: NewDelayEstimator(),
		farBuffer:      NewCircularBuffer(MaxEchoDelay * 2),
		stats:          NewEchoStats(),
		params:         params,
		enabled:        params.Enabled,
		initFrames:     params.InitializationFrames,
	}
}

// ProcessFrame - 处理一帧音频（20ms@48kHz int16 PCM）
func (aec *RealTimeEchoCancel) ProcessFrame(nearEnd []int16) []int16 {
	aec.mu.Lock()
	defer aec.mu.Unlock()

	// 检查输入
	if len(nearEnd) != FrameSize {
		return nearEnd
	}

	// 如果不启用，直接返回
	if !aec.enabled {
		return nearEnd
	}

	// 转换为浮点
	nearFloat := Int16ToFloat64(nearEnd)

	// 初始化阶段，不进行回声消除
	if !aec.initialized {
		aec.initFrames--
		if aec.initFrames <= 0 {
			aec.initialized = true
		}
		return nearEnd
	}

	farFloat := aec.farBuffer.Read(FrameSize)
	// 估计延时
	delay := aec.delayEstimator.Estimate(farFloat, nearFloat)

	farFloat = aec.delayEstimator.AdjustDelay(farFloat, delay)

	// 处理回声消除
	outputFloat := aec.filter.Process(farFloat, nearFloat)

	// 更新统计
	aec.stats.Update(farFloat, nearFloat, outputFloat, delay, false)

	// 转换为int16并返回
	return Float64ToInt16(outputFloat)
}

// AddFarEnd - 单独添加远端信号（用于异步处理）
func (aec *RealTimeEchoCancel) AddFarEnd(farEnd []int16) {
	aec.mu.Lock()
	defer aec.mu.Unlock()

	if len(farEnd) == FrameSize {
		farFloat := Int16ToFloat64(farEnd)
		aec.farBuffer.Write(farFloat)
		aec.delayEstimator.farHistory.Write(farFloat)
	}
}

// Enable/Disable - 启用/禁用回声消除
func (aec *RealTimeEchoCancel) Enable() {
	aec.mu.Lock()
	defer aec.mu.Unlock()
	aec.enabled = true
}

func (aec *RealTimeEchoCancel) Disable() {
	aec.mu.Lock()
	defer aec.mu.Unlock()
	aec.enabled = false
}

func (aec *RealTimeEchoCancel) IsEnabled() bool {
	aec.mu.RLock()
	defer aec.mu.RUnlock()
	return aec.enabled
}

// Reset - 重置回声消除器
func (aec *RealTimeEchoCancel) Reset() {
	aec.mu.Lock()
	defer aec.mu.Unlock()

	//aec.filter.Reset()
	aec.delayEstimator.Reset()
	aec.farBuffer.Clear()
	aec.initialized = false
	aec.initFrames = aec.params.InitializationFrames
	aec.stats = NewEchoStats()
}

// GetStats - 获取统计信息
func (aec *RealTimeEchoCancel) GetStats() EchoStats {
	aec.mu.RLock()
	defer aec.mu.RUnlock()
	return *aec.stats
}

// SetParams - 动态更新参数
func (aec *RealTimeEchoCancel) SetParams(params *EchoParams) {
	aec.mu.Lock()
	defer aec.mu.Unlock()
	aec.params = params
	aec.enabled = params.Enabled
}

// NLMSAEC 基于NLMS（归一化最小均方）算法的回声消除器
type NLMSAEC struct {
	// 滤波器参数
	filterLength   int       // 滤波器长度（建议：64-256ms的回声尾长度）
	filterCoeffs   []float64 // 滤波器系数
	filterHistory  []float64 // 参考信号历史
	stepSize       float64   // 步长 (0.1-0.5)
	regularization float64   // 正则化参数 (1e-6-1e-9)

	// 信号参数
	sampleRate int
	channels   int
	frameSize  int // 20ms帧大小 = 960样本

	// 状态变量
	mu          sync.RWMutex
	isAdapting  bool
	convergence float64
	frameCount  int

	// 双讲检测
	doubleTalkThreshold float64
	doubleTalkDetected  bool

	// 残留回声抑制
	residualEchoSuppressor *ResidualEchoSuppressor
}

// NewNLMSAEC 创建新的NLMS回声消除器
func NewNLMSAEC(sampleRate, channels int) *NLMSAEC {
	frameDuration := 0.020 // 20ms
	frameSize := int(float64(sampleRate) * frameDuration)

	// 滤波器长度：假设最大回声延迟128ms
	filterLength := int(float64(sampleRate) * 0.128 * float64(channels)) // 6144个样本

	aec := &NLMSAEC{
		sampleRate:          sampleRate,
		channels:            channels,
		frameSize:           frameSize,
		filterLength:        filterLength,
		filterCoeffs:        make([]float64, filterLength),
		filterHistory:       make([]float64, filterLength),
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

func (aec *NLMSAEC) Process(reference, mic []float64) []float64 {
	frameSize := len(reference)
	output := make([]float64, frameSize)

	// 更新参考信号历史
	aec.updateReferenceHistory(reference)
	for n := 0; n < frameSize; n++ {
		// 构建当前输入向量
		inputVector := aec.getInputVector(n + frameSize - 1)

		// 计算回声估计
		echoEstimate := 0.0
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
		//output[n] = error
	}

	aec.frameCount++
	return output
}

// 工具函数
func split(data []float64, channels int) [][]float64 {
	frameSize := len(data) / channels
	result := make([][]float64, channels)

	for ch := 0; ch < channels; ch++ {
		result[ch] = make([]float64, frameSize)
		for i := 0; i < frameSize; i++ {
			idx := i*channels + ch
			result[ch][i] = data[idx]
		}
	}

	return result
}

func combine(data [][]float64, channels int) []float64 {
	frameSize := len(data[0])
	result := make([]float64, frameSize*channels)

	for i := 0; i < frameSize; i++ {
		for ch := 0; ch < channels; ch++ {
			idx := i*channels + ch
			val := data[ch][i]
			result[idx] = val
		}
	}

	return result
}

// EchoMetrics 回声消除性能指标
type EchoMetrics struct {
	ERLE         float64 // 回声返回损耗增强 (dB)
	ResidualEcho float64 // 残留回声水平
	Convergence  float64 // 收敛度 (0-1)
}

// updateReferenceHistory 更新参考信号历史
func (aec *NLMSAEC) updateReferenceHistory(reference []float64) {
	// 将新帧添加到历史缓冲区
	copy(aec.filterHistory, aec.filterHistory[len(reference):])
	copy(aec.filterHistory[aec.filterLength-len(reference):], reference)
}

// getInputVector 获取输入向量
func (aec *NLMSAEC) getInputVector(offset int) []float64 {
	vector := make([]float64, aec.filterLength)
	for i := 0; i < aec.filterLength; i++ {
		idx := offset - i
		if idx >= 0 && idx < aec.filterLength {
			vector[i] = aec.filterHistory[idx]
		}
	}
	return vector
}

// updateFilterCoeffs 更新滤波器系数（NLMS算法）
func (aec *NLMSAEC) updateFilterCoeffs(inputVector []float64, error float64) {
	// 计算输入向量的功率
	inputPower := 0.0
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
func (aec *NLMSAEC) detectDoubleTalk(micSample, echoEstimate float64) bool {
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
func (aec *NLMSAEC) updateConvergence(error, inputPower float64) {
	errorPower := error * error
	erle := 10 * math.Log10((inputPower+1e-10)/(errorPower+1e-10))

	// 平滑收敛度估计
	alpha := 0.95
	aec.convergence = alpha*aec.convergence + (1-alpha)*math.Min(1.0, erle/30.0)
}

// GetMetrics 获取性能指标
func (aec *NLMSAEC) GetMetrics() EchoMetrics {
	aec.mu.RLock()
	defer aec.mu.RUnlock()

	return EchoMetrics{
		ERLE:         20 * math.Log10(1/(1-aec.convergence+1e-10)),
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
	suppressionGain float64
	residualLevel   float64
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
func (res *ResidualEchoSuppressor) ProcessSample(error, echoEstimate float64) float64 {
	// 估计噪声和残留回声
	noiseLevel := res.noiseEstimator.Estimate(error)
	residualEcho := res.echoEstimator.Estimate(echoEstimate)

	// 计算后置滤波增益
	signalPower := error*error + 1e-10
	echoPower := residualEcho*residualEcho + 1e-10
	noisePower := noiseLevel*noiseLevel + 1e-10

	// 计算先验SNR和后验SNR
	posteriorSNR := signalPower / (echoPower + noisePower)
	priorSNR := 0.98*res.echoEstimator.lastPriorSNR + 0.02*math.Max(0, posteriorSNR-1)
	res.echoEstimator.lastPriorSNR = priorSNR

	// 计算增益（MMSE-STSA增益）
	gain := priorSNR / (1 + priorSNR)

	// 应用抑制
	suppressed := error * math.Sqrt(gain)

	// 更新残留回声水平
	res.residualLevel = 0.95*res.residualLevel + 0.05*math.Sqrt(echoPower)

	return suppressed
}

// GetResidualLevel 获取残留回声水平
func (res *ResidualEchoSuppressor) GetResidualLevel() float64 {
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
	noiseLevel float64
	minNoise   float64
	smoothing  float64
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
func (ne *NoiseEstimator) Estimate(signal float64) float64 {
	signalPower := signal * signal

	// 使用最小值统计法更新噪声估计
	if ne.frameCount < 100 {
		// 初始阶段直接平均
		ne.noiseLevel = (ne.noiseLevel*float64(ne.frameCount) + signalPower) / float64(ne.frameCount+1)
	} else {
		// 平滑更新
		alpha := ne.smoothing
		if signalPower < ne.noiseLevel {
			alpha = 0.99 // 噪声降低时慢速更新
		}
		ne.noiseLevel = alpha*ne.noiseLevel + (1-alpha)*signalPower
	}

	ne.frameCount++
	ne.noiseLevel = math.Max(ne.noiseLevel, ne.minNoise)

	return math.Sqrt(ne.noiseLevel)
}

// Reset 重置噪声估计器
func (ne *NoiseEstimator) Reset() {
	ne.noiseLevel = 1e-4
	ne.frameCount = 0
}

// EchoEstimator 回声估计器
type EchoEstimator struct {
	echoLevel    float64
	lastEcho     float64
	lastPriorSNR float64
	smoothing    float64
}

// NewEchoEstimator 创建回声估计器
func NewEchoEstimator(sampleRate int) *EchoEstimator {
	return &EchoEstimator{
		echoLevel: 1e-4,
		smoothing: 0.9,
	}
}

// Estimate 估计回声水平
func (ee *EchoEstimator) Estimate(echoEstimate float64) float64 {
	echoPower := echoEstimate * echoEstimate

	// 平滑更新回声估计
	alpha := ee.smoothing
	if echoPower < ee.echoLevel {
		alpha = 0.95 // 回声降低时慢速更新
	}
	ee.echoLevel = alpha*ee.echoLevel + (1-alpha)*echoPower
	ee.lastEcho = echoEstimate

	return math.Sqrt(ee.echoLevel)
}

// Reset 重置回声估计器
func (ee *EchoEstimator) Reset() {
	ee.echoLevel = 1e-4
	ee.lastEcho = 0.0
	ee.lastPriorSNR = 0.0
}
