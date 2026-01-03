package view

import (
	"bytes"
	"context"
	"errors"
	"image"
	"io"
	"log"
	"rtc/assets/fonts"
	"rtc/internal/audio"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

type VoiceRecorder struct {
	audio.StreamConfig
	InteractiveSpan
	ExpandButton
	ctx    context.Context
	cancel context.CancelFunc
	buf    *bytes.Buffer
}

func (v *VoiceRecorder) Layout(gtx layout.Context) layout.Dimensions {
	for {
		e, ok := v.Update(gtx)
		if !ok {
			break
		}
		if e.Type == LongPress {
			log.Printf("long pressing start")
			go func() {
				v.ctx, v.cancel = context.WithCancel(context.Background())
				v.buf = new(bytes.Buffer)
				err := audio.Capture(v.ctx, v.buf, v.StreamConfig)
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
					log.Printf("capture audio failed, %s", err)
					v.cancel()
				}
			}()
		}
		if e.Type == Click {
			log.Printf("long pressing end")
			go func() {
				if v.buf == nil {
					return
				}
				v.cancel()
				reader := bytes.NewReader(v.buf.Bytes())
				if err := audio.Playback(context.Background(), reader, v.StreamConfig); err != nil && !errors.Is(err, io.EOF) {
					log.Printf("audio playback: %w", err)
				}
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
				bgColor := fonts.DefaultTheme.ContrastBg
				defer clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Max}, gtx.Dp(4)).Push(gtx.Ops).Pop()
				paint.Fill(gtx.Ops, bgColor)
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}),
			// expand button
			layout.Rigid(v.ExpandButton.Layout),
		)
	})
}
