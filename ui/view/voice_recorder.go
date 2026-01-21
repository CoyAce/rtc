package view

import (
	"bytes"
	"context"
	"errors"
	"image"
	"log"
	"os"
	"path/filepath"
	"rtc/assets/fonts"
	"rtc/core"
	"rtc/internal/audio"
	"rtc/ui/native"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"github.com/CoyAce/opus"
	"github.com/CoyAce/opus/ogg"
)

type VoiceRecorder struct {
	audio.StreamConfig
	InteractiveSpan
	ExpandButton
	cancel context.CancelFunc
	buf    *bytes.Buffer
}

func (v *VoiceRecorder) Layout(gtx layout.Context) layout.Dimensions {
	bgColor := fonts.DefaultTheme.ContrastBg
	if v.longPressing {
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
		}
		if e.Type == LongPressRelease {
			v.cancel()
			v.encodeAndSendAsync()
		}
		if e.Type == Click {
			v.cancel()
		}
	}
	margins := layout.Inset{Top: unit.Dp(8.0), Left: unit.Dp(8.0), Right: unit.Dp(8), Bottom: unit.Dp(15)}
	return margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Spacing:   layout.SpaceBetween,
			Alignment: layout.Middle,
		}.Layout(gtx,
			// voice input
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.Y = gtx.Dp(42)
				v.InteractiveSpan.Layout(gtx)
				defer clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Max}, gtx.Dp(21)).Push(gtx.Ops).Pop()
				paint.Fill(gtx.Ops, bgColor)
				layout.Inset{Top: unit.Dp(42 * 0.15 / 2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Stack{Alignment: layout.Center}.Layout(gtx,
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = int(float32(gtx.Constraints.Max.Y) * 0.85)
							return voiceMessageIcon.Layout(gtx, fonts.DefaultTheme.ContrastFg)
						}),
					)
				})
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}),
			// expand button
			layout.Rigid(v.ExpandButton.Layout),
		)
	})
}

func (v *VoiceRecorder) encodeAndSendAsync() {
	go func() {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		timeNow := time.Now().In(loc).Format("20060102150405")
		filePath := core.GetDataPath(timeNow + ".opus")
		log.Printf("audio filePath %s", filePath)
		w, err := os.Create(filePath)
		if err != nil {
			log.Printf("create file %s failed, %s", filePath, err)
			return
		}
		pcm := v.buf.Bytes()
		samples := len(pcm) / 2
		processed, err := enhancer.ProcessAudio(audio.Int16ToFloat32(ogg.ToInts(pcm)))
		if err != nil {
			log.Printf("process audio failed, %s", err)
		}
		output := ogg.ToBytes(audio.Float32ToInt16(processed))
		err = ogg.NewEncoder(ogg.FrameSize, 1, opus.AppVoIP).Encode(w, output)
		if err != nil {
			log.Printf("encode file %s failed, %s", filePath, err)
		}
		v.buf = nil
		duration := uint64(ogg.GetDuration(samples) / time.Millisecond)
		message := Message{
			State: Stateless,
			Theme: fonts.DefaultTheme,
			UUID:  core.DefaultClient.FullID(),
			Type:  Voice, Filename: filepath.Base(filePath),
			Sender:       core.DefaultClient.FullID(),
			CreatedAt:    time.Now(),
			MediaControl: MediaControl{StreamConfig: v.StreamConfig, Duration: duration},
		}
		MessageBox <- &message
		err = core.DefaultClient.SendVoice(filepath.Base(filePath), duration)
		if err != nil {
			log.Printf("Send voice %s failed, %s", filePath, err)
		} else {
			message.State = Sent
		}
	}()
}

func (v *VoiceRecorder) recordAsync() {
	go func() {
		native.DefaultRecorder.AskPermission()
		var ctx context.Context
		ctx, v.cancel = context.WithCancel(context.Background())
		v.buf = new(bytes.Buffer)
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
