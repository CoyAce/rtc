package view

import (
	"bytes"
	"context"
	"errors"
	"image"
	"log"
	"os"
	"rtc/assets/fonts"
	"rtc/core"
	"rtc/internal/audio"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
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
		if e.Type == LongPress {
			log.Printf("long pressing start")
			go func() {
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
		if e.Type == LongPressRelease {
			log.Printf("long pressing end")
			v.cancel()
			go func() {
				timeNow := time.Now().Local().Format("20060104150405")
				filePath := core.GetDataPath(timeNow + ".opus")
				log.Printf("audio filePath: %s", filePath)
				w, err := os.Create(filePath)
				defer w.Close()
				if err != nil {
					log.Printf("create file %s failed, %s", filePath, err)
					return
				}
				pcm := v.buf.Bytes()
				samples := len(pcm)
				err = ogg.Encode(w, pcm)
				if err != nil {
					log.Printf("encode file %s failed, %s", filePath, err)
				}
				v.buf = nil
				message := Message{
					State: Stateless,
					Theme: fonts.DefaultTheme,
					UUID:  core.DefaultClient.FullID(),
					Type:  Voice, Filename: filePath,
					Sender:       core.DefaultClient.FullID(),
					CreatedAt:    time.Now(),
					MediaControl: MediaControl{StreamConfig: v.StreamConfig},
					Size:         int(ogg.GetDuration(samples/2/ogg.Channels) / time.Millisecond),
				}
				MessageBox <- &message
			}()
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
				defer clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Max}, gtx.Dp(4)).Push(gtx.Ops).Pop()
				paint.Fill(gtx.Ops, bgColor)
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}),
			// expand button
			layout.Rigid(v.ExpandButton.Layout),
		)
	})
}
