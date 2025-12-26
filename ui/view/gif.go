package view

import (
	"image"
	"image/color"
	"image/gif"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/x/component"
)

type Gif struct {
	*gif.GIF
	index     int
	nextFrame time.Time
}

func (g *Gif) Layout(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Max = image.Point{X: g.Config.Width, Y: g.Config.Height}
	delay := time.Duration(10*g.Delay[g.index]) * time.Millisecond
	if g.nextFrame == (time.Time{}) {
		g.nextFrame = time.Now().Add(delay)
	}
	if !time.Now().Before(g.nextFrame) {
		g.index = (g.index + 1) % len(g.Image)
		delay = time.Duration(10*g.Delay[g.index]) * time.Millisecond
		g.nextFrame = time.Now().Add(delay)
	}
	// other frames
	for i := 0; i < g.index; i++ {
		img := g.Image[i]
		switch g.Disposal[i] {
		default:
			fallthrough
		case gif.DisposalNone:
			// 不做特殊处理，下一帧直接叠加在当前帧之上
			transition := op.Offset(img.Rect.Min).Push(gtx.Ops)
			paint.NewImageOp(img).Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			transition.Pop()
		case gif.DisposalBackground:
			// 清除当前帧覆盖的区域，并用背景色填充
			var bgColor color.Color = color.Transparent
			if palette, ok := g.Config.ColorModel.(color.Palette); ok {
				bgColor = palette[g.BackgroundIndex]
			}
			// background
			R, G, B, A := bgColor.RGBA()
			transition := op.Offset(img.Rect.Min).Push(gtx.Ops)
			component.Rect{Color: color.NRGBA{uint8(R), uint8(G), uint8(B), uint8(A)},
				Size: img.Bounds().Size()}.Layout(gtx)
			transition.Pop()
		case gif.DisposalPrevious:
			continue
		}
	}
	// current frame
	img := g.Image[g.index]
	transition := op.Offset(img.Rect.Min).Push(gtx.Ops)
	paint.NewImageOp(img).Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	transition.Pop()

	nextFrame := gtx.Now.Add(delay)
	gtx.Execute(op.InvalidateCmd{At: nextFrame})

	return layout.Dimensions{Size: gtx.Constraints.Max}
}
