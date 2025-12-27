package view

import (
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
)

type Gif struct {
	*gif.GIF
	index     int
	nextFrame time.Time
	frames    []image.Image
}

func (g *Gif) Layout(gtx layout.Context) layout.Dimensions {
	v := gtx.Constraints.Min.X
	if g.Config.Width < g.Config.Height {
		gtx.Constraints.Min.X = v
		gtx.Constraints.Min.Y = int(float32(g.Config.Height) / float32(g.Config.Width) * float32(v))
	} else {
		gtx.Constraints.Min.Y = v
		gtx.Constraints.Min.X = int(float32(g.Config.Width) / float32(g.Config.Height) * float32(v))
	}

	delay := time.Duration(10*g.Delay[g.index]) * time.Millisecond
	if g.nextFrame == (time.Time{}) {
		g.nextFrame = time.Now().Add(delay)
	}
	if !time.Now().Before(g.nextFrame) {
		g.index = (g.index + 1) % len(g.Image)
		delay = time.Duration(10*g.Delay[g.index]) * time.Millisecond
		g.nextFrame = time.Now().Add(delay)
	}

	scale := f32.Point{
		X: float32(gtx.Constraints.Min.X) / float32(g.Config.Width), Y: float32(gtx.Constraints.Min.Y) / float32(g.Config.Height),
	}
	//defer clip.Rect{Max: gtx.Constraints.Min}.Push(gtx.Ops).Pop()
	defer op.Affine(f32.AffineId().Scale(f32.Point{}, scale)).Push(gtx.Ops).Pop()

	currentFrameCached := len(g.frames) > g.index
	if currentFrameCached {
		paint.NewImageOp(g.frames[g.index]).Add(gtx.Ops)
	} else {
		canvas := g.extractFrame()
		g.frames = append(g.frames, canvas)

		paint.NewImageOp(canvas).Add(gtx.Ops)
	}
	paint.PaintOp{}.Add(gtx.Ops)

	nextFrame := gtx.Now.Add(delay)
	gtx.Execute(op.InvalidateCmd{At: nextFrame})

	return layout.Dimensions{Size: gtx.Constraints.Min}
}

func (g *Gif) extractFrame() *image.RGBA {
	canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	prev := g.index - 1
LOOP:
	if prev >= 0 {
		prevFrame := g.frames[prev]
		switch g.Disposal[prev] {
		default:
			fallthrough
		case gif.DisposalNone:
			// 不做特殊处理，直接叠加
			draw.Draw(canvas, prevFrame.Bounds(), prevFrame, image.Point{}, draw.Over)
		case gif.DisposalBackground:
			// 清除帧覆盖的区域，并用背景色填充
			draw.Draw(canvas, prevFrame.Bounds(), prevFrame, image.Point{}, draw.Over)
			var bgColor color.Color = color.Transparent
			if palette, ok := g.Config.ColorModel.(color.Palette); ok {
				bgColor = palette[g.BackgroundIndex]
			}
			draw.Draw(canvas, g.Image[prev].Bounds(), &image.Uniform{C: bgColor}, g.Image[prev].Bounds().Min, draw.Src)
		case gif.DisposalPrevious:
			// 恢复到前一帧的状态
			prev--
			goto LOOP
		}
	}
	// current frame
	frame := g.Image[g.index]
	draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
	return canvas
}
