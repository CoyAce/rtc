package view

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"log"
	"math"
	"mushin/assets/fonts"
	"mushin/assets/icons"
	"mushin/internal/audio"
	"mushin/ui/native"
	"os"
	"path/filepath"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"github.com/CoyAce/opus"
	"github.com/CoyAce/opus/ogg"
	"github.com/CoyAce/wi"
	"github.com/gen2brain/malgo"
)

type VoiceRecorder struct {
	audio.StreamConfig
	InteractiveSpan
	ExpandButton
	cancel    context.CancelFunc
	buf       *bytes.Buffer
	startTime time.Time
	waveform  []float32
}

func (v *VoiceRecorder) Layout(gtx layout.Context) layout.Dimensions {
	// Initialize recording state
	if v.longPressing && v.startTime.IsZero() {
		v.startTime = gtx.Now
		v.waveform = make([]float32, 0)
	}

	// Animate background when recording
	bgColor := fonts.DefaultTheme.ContrastBg
	if v.longPressing {
		// Pulsing neon effect during recording
		elapsed := float32(time.Now().Sub(v.startTime).Seconds())
		pulse := (float32(math.Sin(float64(elapsed*4*math.Pi)))+1)/2 + 0.5 // 0.5 to 1.5
		bgColor.R = uint8(float32(bgColor.R) * pulse)
		bgColor.G = uint8(float32(bgColor.G) * pulse)
		bgColor.B = uint8(float32(bgColor.B) * pulse)
		bgColor.A = 216
	}

	for {
		e, ok := v.Update(gtx)
		if !ok {
			break
		}
		if e.Type == Press {
			v.recordAsync()
		}
		if e.Type == LongPress {
			gtx.Execute(op.InvalidateCmd{})
		} else if e.Type == LongPressRelease {
			v.cancel()
			v.encodeAndSendAsync()
		} else if e.Type == Click || e.Type == Cancel {
			v.cancel()
		}
	}
	// Ensure continuous animation for cancellation feedback
	if v.longPressing {
		gtx.Execute(op.InvalidateCmd{})
	}
	macro := op.Record(gtx.Ops)
	margins := layout.Inset{Top: unit.Dp(8.0), Left: unit.Dp(8.0), Right: unit.Dp(8), Bottom: unit.Dp(15)}
	dimensions := margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Spacing:   layout.SpaceBetween,
			Alignment: layout.Middle,
		}.Layout(gtx,
			// voice input with geek-style waveform visualization
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.Y = gtx.Dp(42)
				v.InteractiveSpan.Layout(gtx)

				// Record drawing operations for rounded rectangle with effects
				defer clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Max}, gtx.Dp(21)).Push(gtx.Ops).Pop()

				// Draw waveform visualization when recording
				if v.longPressing && v.click.Hovered() {
					v.drawWaveform(gtx)
				} else if v.longPressing && !v.click.Hovered() {
					op.Offset(image.Pt(gtx.Dp(50/2), 0)).Add(gtx.Ops)
					// Show cancellation feedback
					v.drawCancelledIndicator(gtx)
				} else {
					op.Offset(image.Pt(gtx.Dp(50/2), 0)).Add(gtx.Ops)
					// Draw centered icon when not recording
					layout.Inset{Top: unit.Dp(42 * 0.15 / 2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Stack{Alignment: layout.Center}.Layout(gtx,
							layout.Stacked(func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min.X = int(float32(gtx.Constraints.Max.Y) * 0.85)
								iconColor := fonts.DefaultTheme.ContrastFg
								iconColor.A = 200
								return icons.VoiceMessageIcon.Layout(gtx, iconColor)
							}),
						)
					})
				}

				return layout.Dimensions{Size: gtx.Constraints.Max}
			}),
			// expand button
			layout.Rigid(v.ExpandButton.Layout),
		)
	})
	call := macro.Stop()

	// Draw rounded background with geek-style gradient and glow effects
	radius := gtx.Dp(20)

	defer clip.RRect{
		Rect: image.Rectangle{Max: dimensions.Size},
		NE:   radius, NW: radius, SE: radius, SW: radius,
	}.Push(gtx.Ops).Pop()

	// Add cyberpunk glow effects when recording
	var topGlowColor color.NRGBA
	var bottomGlowColor color.NRGBA

	if v.longPressing && v.click.Hovered() {
		// Normal recording state - cyan glow
		topGlowColor = fonts.BrightCyan
		topGlowColor.A = 180

		// Bottom breathing glow (purple to cyan gradient effect)
		elapsed := time.Since(v.startTime).Seconds()
		breathFactor := (float32(math.Sin(elapsed*math.Pi)) + 1) / 2
		bottomGlowColor = fonts.BrightPurple
		bottomGlowColor.A = 120 + uint8(float32(80)*breathFactor)
	} else if v.longPressing && !v.click.Hovered() {
		// Canceled state - red warning glow (continuous flashing)
		elapsed := time.Since(v.startTime).Seconds()
		// Fast continuous flashing: oscillate between 0 and 1 every 1 seconds
		flashFactor := float32(math.Abs(math.Sin(elapsed * math.Pi)))
		topGlowColor = color.NRGBA{R: 255, G: 50, B: 50, A: uint8(200 * flashFactor)}
		bottomGlowColor = color.NRGBA{R: 255, G: 100, B: 100, A: uint8(180 * flashFactor)}
	} else {
		// Idle state - subtle glow
		topGlowColor = fonts.BrightCyan
		topGlowColor.A = 60
		bottomGlowColor = fonts.BrightPurple
		bottomGlowColor.A = 40
	}

	// Top neon line (cyan glow)
	glowLineHeight := gtx.Dp(1)
	paint.FillShape(gtx.Ops, topGlowColor, clip.Rect{
		Min: image.Point{Y: 0},
		Max: image.Point{X: dimensions.Size.X, Y: glowLineHeight},
	}.Op())

	baseGlowHeight := gtx.Dp(3)
	paint.FillShape(gtx.Ops, bottomGlowColor, clip.Rect{
		Min: image.Point{Y: dimensions.Size.Y - baseGlowHeight},
		Max: image.Point{X: dimensions.Size.X, Y: dimensions.Size.Y},
	}.Op())

	call.Add(gtx.Ops)
	return dimensions
}

func (v *VoiceRecorder) drawWaveform(gtx layout.Context) {
	// Simulate real-time waveform based on recording time
	elapsed := float32(time.Now().Sub(v.startTime).Seconds())
	barCount := gtx.Constraints.Max.X / gtx.Dp(3)

	if barCount > 0 {
		centerY := gtx.Constraints.Max.Y / 2
		maxBarHeight := gtx.Dp(28)

		// Draw animated waveform bars
		for i := 0; i < int(barCount); i++ {
			// Calculate bar position with left-to-right animation
			progress := float32(i) / float32(barCount)
			animationDelay := elapsed - progress*0.5

			if animationDelay < 0 {
				continue // Bars haven't reached this position yet
			}

			// Generate bar height using sine wave combination for organic look
			phase := float32(i) * 0.3
			barHeight := float32(maxBarHeight) * float32(
				0.3+
					0.4*math.Abs(math.Sin(float64(phase+elapsed*2)))+
					0.3*math.Abs(math.Sin(float64(phase*2-elapsed*3))),
			)

			// Gradient color from cyan to purple
			var barColor color.NRGBA
			if i%2 == 0 {
				barColor = fonts.BrightCyan
				barColor.A = uint8(float32(180) * float32(math.Sin(float64(animationDelay*math.Pi))))
			} else {
				barColor = fonts.BrightPurple
				barColor.A = uint8(float32(180) * float32(math.Sin(float64(animationDelay*math.Pi))))
			}

			// Draw bar centered vertically
			barWidth := gtx.Dp(2)
			x := i * gtx.Dp(3)
			y := centerY - int(barHeight)/2

			paint.FillShape(gtx.Ops, barColor, clip.Rect{
				Min: image.Point{X: x, Y: y},
				Max: image.Point{X: x + barWidth, Y: y + int(barHeight)},
			}.Op())
		}
	}
}

func (v *VoiceRecorder) drawCancelledIndicator(gtx layout.Context) {
	// Draw a geek-style glitch/broken waveform effect
	elapsed := time.Since(v.startTime).Seconds()

	// Fast pulsing for urgency
	pulseFactor := float32(math.Abs(math.Sin(elapsed * math.Pi)))

	centerX := gtx.Constraints.Max.X / 2
	centerY := gtx.Constraints.Max.Y / 2
	barWidth := gtx.Dp(3)
	maxBarHeight := gtx.Dp(20)

	// Draw broken/scattered waveform bars (glitch effect)
	barCount := 8
	for i := 0; i < barCount; i++ {
		// Calculate offset from center with glitch randomness
		phase := float32(i) * 0.5
		glitchOffset := float32(math.Sin(float64(elapsed*10)+float64(phase))) * 20 * pulseFactor

		// Alternating bar heights for broken effect
		barHeight := float32(maxBarHeight) * float32(
			0.3+0.7*math.Abs(math.Sin(float64(phase*2)+elapsed*3)),
		)

		// Red to cyan gradient based on position
		var barColor color.NRGBA
		if i%2 == 0 {
			// Bright red
			barColor = color.NRGBA{
				R: 255,
				G: uint8(50 * pulseFactor),
				B: uint8(50 * pulseFactor),
				A: uint8(180 * pulseFactor),
			}
		} else {
			// Cyan accent (complementary to red)
			barColor = color.NRGBA{
				R: uint8(50 * pulseFactor),
				G: uint8(200 * pulseFactor),
				B: 255,
				A: uint8(150 * pulseFactor),
			}
		}

		// Position bars in a broken cross pattern
		x := centerX - int(float32(barCount)*float32(barWidth)/2) + i*int(barWidth) + int(glitchOffset)
		y := centerY - int(barHeight)/2

		paint.FillShape(gtx.Ops, barColor, clip.Rect{
			Min: image.Point{X: x, Y: y},
			Max: image.Point{X: x + barWidth, Y: y + int(barHeight)},
		}.Op())
	}

	// Draw corner brackets (like a targeting reticle breaking apart)
	bracketSize := gtx.Dp(12)
	bracketThickness := gtx.Dp(2)
	bracketColor := color.NRGBA{R: 255, G: 100, B: 100, A: uint8(200 * pulseFactor)}

	// Four corner brackets that pulse outward
	expandFactor := 1.0 + 0.3*pulseFactor

	// Top-left bracket
	paint.FillShape(gtx.Ops, bracketColor, clip.Rect{
		Min: image.Point{
			X: centerX - int(float32(bracketSize)*expandFactor),
			Y: centerY - int(float32(bracketSize)*expandFactor),
		},
		Max: image.Point{
			X: centerX - int(float32(bracketSize)*expandFactor) + int(bracketThickness),
			Y: centerY - int(float32(bracketSize)*expandFactor) + int(bracketThickness),
		},
	}.Op())

	// Top-right bracket
	paint.FillShape(gtx.Ops, bracketColor, clip.Rect{
		Min: image.Point{
			X: centerX + int(float32(bracketSize)*expandFactor) - int(bracketThickness),
			Y: centerY - int(float32(bracketSize)*expandFactor),
		},
		Max: image.Point{
			X: centerX + int(float32(bracketSize)*expandFactor),
			Y: centerY - int(float32(bracketSize)*expandFactor) + int(bracketThickness),
		},
	}.Op())

	// Bottom-left bracket
	paint.FillShape(gtx.Ops, bracketColor, clip.Rect{
		Min: image.Point{
			X: centerX - int(float32(bracketSize)*expandFactor),
			Y: centerY + int(float32(bracketSize)*expandFactor) - int(bracketThickness),
		},
		Max: image.Point{
			X: centerX - int(float32(bracketSize)*expandFactor) + int(bracketThickness),
			Y: centerY + int(float32(bracketSize)*expandFactor),
		},
	}.Op())

	// Bottom-right bracket
	paint.FillShape(gtx.Ops, bracketColor, clip.Rect{
		Min: image.Point{
			X: centerX + int(float32(bracketSize)*expandFactor) - int(bracketThickness),
			Y: centerY + int(float32(bracketSize)*expandFactor) - int(bracketThickness),
		},
		Max: image.Point{
			X: centerX + int(float32(bracketSize)*expandFactor),
			Y: centerY + int(float32(bracketSize)*expandFactor),
		},
	}.Op())
}

func (v *VoiceRecorder) encodeAndSendAsync() {
	go func() {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		timeNow := time.Now().In(loc).Format("20060102150405")
		filePath := GetDataPath(timeNow + ".opus")
		log.Printf("audio file path %s", filePath)
		w, err := os.Create(filePath)
		if err != nil {
			log.Printf("create file %s failed, %s", filePath, err)
			return
		}
		pcm := v.buf.Bytes()
		samples := len(pcm) / 4
		processed, err := enhancer.ProcessBatch(audio.ToFloat32(pcm))
		if err != nil {
			log.Printf("process audio failed, %s", err)
		}
		output := ogg.ToBytes(audio.Float32ToInt16(processed))
		err = ogg.NewEncoder(ogg.FrameSize, 1, opus.AppVoIP).Encode(w, output)
		if err != nil {
			log.Printf("encode file %s failed, %s", filePath, err)
		}
		v.buf = nil
		duration := uint32(ogg.GetDuration(samples) / time.Millisecond)
		message := Message{
			State: Stateless,
			MessageStyle: MessageStyle{
				Theme: fonts.DefaultTheme,
			},
			Contacts:     FromMyself(),
			MessageType:  Voice,
			FileControl:  FileControl{Filename: filepath.Base(filePath)},
			CreatedAt:    time.Now(),
			MediaControl: MediaControl{StreamConfig: v.StreamConfig, Duration: duration},
		}
		message.Format = malgo.FormatS16
		MessageBox <- &message
		err = wi.DefaultClient.SendVoice(filePath, duration)
		if err != nil {
			log.Printf("Send voice %s failed, %s", filePath, err)
		} else {
			message.State = Sent
		}
	}()
}

func (v *VoiceRecorder) recordAsync() {
	go func() {
		native.Tool.AskMicrophonePermission()
		var ctx context.Context
		ctx, v.cancel = context.WithCancel(context.Background())
		v.buf = new(bytes.Buffer)
		v.StreamConfig.Format = malgo.FormatF32
		err := audio.Capture(ctx, v.buf, v.StreamConfig)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			log.Printf("capture audio failed, %s", err)
			v.cancel()
		}
	}()
}
