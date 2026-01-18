package audio

// EchoCanceller 回声消除器接口
type EchoCanceller interface {
	Process(reference, mic []int16) []int16 // 处理一帧数据
	Reset()                                 // 重置状态
	GetMetrics() EchoMetrics                // 获取性能指标
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

	if config.EnableDelayCorrection {
		aec.delayEstimator = NewDelayEstimator()
	}

	if config.EnableNonlinearProcessing {
		aec.nonlinearProcessor = NewNonlinearProcessor(config.SampleRate)
	}

	return aec
}

// Process 处理一帧数据
//func (waec *WebRTCAEC) Process(reference, mic []int16) []int16 {
//	// 1. 延迟估计和校正
//	var correctedReference []int16
//	if waec.delayEstimator != nil {
//		//delay := waec.delayEstimator.EstimateDelay(reference, mic)
//		//correctedReference = waec.delayEstimator.AdjustDelay(reference, delay)
//	} else {
//		correctedReference = reference
//	}
//
//	// 2. NLMS回声消除
//	output := waec.nlmsAEC.Process(correctedReference, mic)
//
//	// 3. 非线性处理（抑制残留回声）
//	if waec.nonlinearProcessor != nil {
//		output = waec.nonlinearProcessor.Process(output, correctedReference)
//	}
//
//	// 4. 更新指标
//	waec.metrics = waec.nlmsAEC.GetMetrics()
//
//	return output
//}

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

//// DelayEstimator 延迟估计器
//type DelayEstimator struct {
//	sampleRate int
//	maxDelay   int // 最大延迟（样本数）
//	bufferSize int
//	referenceBuffer []float64
//	micBuffer      []float64
//	xcorrBuffer    []float64
//}
//
//// NewDelayEstimator 创建延迟估计器
//func NewDelayEstimator(sampleRate int) *DelayEstimator {
//	maxDelayMs := 300 // 最大300ms延迟
//	maxDelay := sampleRate * maxDelayMs / 1000
//	bufferSize := maxDelay * 2
//
//	return &DelayEstimator{
//		sampleRate:     sampleRate,
//		maxDelay:       maxDelay,
//		bufferSize:     bufferSize,
//		referenceBuffer: make([]float64, bufferSize),
//		micBuffer:      make([]float64, bufferSize),
//		xcorrBuffer:    make([]float64, maxDelay*2),
//	}
//}

// EstimateDelay 估计延迟
//
//	func (de *DelayEstimator) EstimateDelay(reference, mic []int16) int {
//		// 转换为浮点数
//		refFloat := split(reference, 1)[0]
//		micFloat := split(mic, 1)[0]
//
//		// 更新缓冲区
//		de.updateBuffers(refFloat, micFloat)
//
//		// 计算互相关
//		de.computeCrossCorrelation()
//
//		// 寻找最大相关位置
//		maxIdx := 0
//		maxCorr := -1.0
//		for i := 0; i < len(de.xcorrBuffer); i++ {
//			if de.xcorrBuffer[i] > maxCorr {
//				maxCorr = de.xcorrBuffer[i]
//				maxIdx = i
//			}
//		}
//
//		// 转换为延迟（样本数）
//		delay := maxIdx - de.maxDelay
//		return delay
//	}
//
// AdjustDelay 调整延迟

//func (de *DelayEstimator) updateBuffers(ref, mic []float64) {
//	// 更新参考信号缓冲区
//	copy(de.referenceBuffer, de.referenceBuffer[len(ref):])
//	copy(de.referenceBuffer[de.bufferSize-len(ref):], ref)
//
//	// 更新麦克风信号缓冲区
//	copy(de.micBuffer, de.micBuffer[len(mic):])
//	copy(de.micBuffer[de.bufferSize-len(mic):], mic)
//}

//func (de *DelayEstimator) computeCrossCorrelation() {
//	// 简化版互相关计算
//	for i := 0; i < len(de.xcorrBuffer); i++ {
//		corr := 0.0
//		offset := i - de.maxDelay
//
//		for j := 0; j < 960; j++ { // 使用一帧数据
//			refIdx := de.bufferSize - 960 + j
//			micIdx := refIdx + offset
//
//			if micIdx >= 0 && micIdx < de.bufferSize {
//				corr += de.referenceBuffer[refIdx] * de.micBuffer[micIdx]
//			}
//		}
//
//		de.xcorrBuffer[i] = corr / 960.0
//	}
//}

//func (de *DelayEstimator) Reset() {
//	for i := range de.referenceBuffer {
//		de.referenceBuffer[i] = 0.0
//	}
//	for i := range de.micBuffer {
//		de.micBuffer[i] = 0.0
//	}
//	for i := range de.xcorrBuffer {
//		de.xcorrBuffer[i] = 0.0
//	}
//}

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
//func (nlp *NonlinearProcessor) Process(signal, reference []int16) []int16 {
//	frameSize := len(signal)
//	signalFloat := split(signal, 1)[0]
//	refFloat := split(reference, 1)[0]
//
//	output := make([]float64, frameSize)
//
//	for i := 0; i < frameSize; i++ {
//		// 估计噪声和回声
//		noiseLevel := nlp.noiseEstimator.Estimate(signalFloat[i])
//		echoLevel := nlp.echoEstimator.Estimate(refFloat[i])
//
//		// 计算信号功率
//		signalPower := signalFloat[i] * signalFloat[i]
//		totalDisturbance := echoLevel*echoLevel + noiseLevel*noiseLevel
//
//		// 计算增益
//		snr := signalPower / (totalDisturbance + 1e-10)
//		gain := snr / (1 + snr)
//
//		// 应用非线性抑制
//		nlp.suppressionGain[i] = 0.9*nlp.suppressionGain[i] + 0.1*gain
//		output[i] = signalFloat[i] * math.Sqrt(nlp.suppressionGain[i])
//	}
//
//	// 转换回int16
//	result := make([]int16, frameSize)
//	for i, val := range output {
//		if val > 1.0 {
//			val = 1.0
//		} else if val < -1.0 {
//			val = -1.0
//		}
//		result[i] = int16(val * 32767.0)
//	}
//
//	return result
//}

func (nlp *NonlinearProcessor) Reset() {
	for i := range nlp.suppressionGain {
		nlp.suppressionGain[i] = 1.0
	}
	nlp.noiseEstimator.Reset()
	nlp.echoEstimator.Reset()
}
