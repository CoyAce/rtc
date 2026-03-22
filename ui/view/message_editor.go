package view

import (
	"image"
	"image/color"
	"io"
	"math"
	"mushin/assets/fonts"
	"mushin/assets/icons"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/io/clipboard"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/CoyAce/wi"
)

type MessageEditor struct {
	*material.Theme
	InteractiveSpan
	EditorOperator
	ExpandButton
	widget.Editor
	submitButton widget.Clickable
	startTime    time.Time
	focused      bool
}

func (e *MessageEditor) Layout(gtx layout.Context) layout.Dimensions {
	e.update(gtx)
	if e.operationBarNeeded(gtx) {
		e.EditorOperator.Layout(gtx)
	}
	//defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	e.InteractiveSpan.Layout(gtx)

	// Draw rounded background with geek-style gradient border effect at outer layer
	macro := op.Record(gtx.Ops)
	margins := layout.Inset{Top: unit.Dp(8.0), Left: unit.Dp(8.0), Right: unit.Dp(8), Bottom: unit.Dp(15)}
	dimensions := margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Spacing:   layout.SpaceBetween,
			Alignment: layout.Middle,
		}.Layout(gtx,
			// text input
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				innerMargins := layout.Inset{Left: unit.Dp(20), Right: unit.Dp(12)}
				return innerMargins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.Editor(e.Theme, &e.Editor, "Messages").Layout(gtx)
				})
			}),
			// submit button
			layout.Rigid(e.drawSubmitButton),
			// expand button
			layout.Rigid(e.ExpandButton.Layout),
		)
	})
	call := macro.Stop()

	// Draw rounded background with geek-style gradient and glow effects
	radius := gtx.Dp(20)

	defer clip.RRect{
		Rect: image.Rectangle{Max: dimensions.Size},
		NE:   radius, NW: radius, SE: radius, SW: radius,
	}.Push(gtx.Ops).Pop()

	// 1. Render solid dark background (remove subtle gradient - not visible on black)
	bgColor := color.NRGBA{R: 10, G: 15, B: 30, A: 220}
	paint.FillShape(gtx.Ops, bgColor, clip.Rect{Max: dimensions.Size}.Op())

	// 2. Add cyan glow line at top edge for cyberpunk feel
	glowLineHeight := gtx.Dp(1)
	topGlowColor := fonts.DefaultTheme.ContrastBg
	topGlowColor.A = 100
	paint.FillShape(gtx.Ops, topGlowColor, clip.Rect{
		Min: image.Point{Y: 0},
		Max: image.Point{X: dimensions.Size.X, Y: glowLineHeight},
	}.Op())

	// 3. Add breathing glow effect at bottom
	baseGlowHeight := gtx.Dp(3)

	// Calculate breathing factor (sine wave, 2 second period)
	if e.startTime.IsZero() {
		e.startTime = time.Now()
	}
	elapsed := time.Now().Sub(e.startTime).Seconds()
	breathFactor := (math.Sin(elapsed*math.Pi) + 1) / 2 // 0 to 1

	breathingOpacity := uint8(float32(150) + float32(80)*float32(breathFactor))
	bottomGlowColor := fonts.DefaultTheme.ContrastBg
	bottomGlowColor.A = breathingOpacity
	paint.FillShape(gtx.Ops, bottomGlowColor, clip.Rect{
		Min: image.Point{Y: dimensions.Size.Y - baseGlowHeight},
		Max: image.Point{X: dimensions.Size.X, Y: dimensions.Size.Y},
	}.Op())

	call.Add(gtx.Ops)
	return dimensions
}

func (e *MessageEditor) update(gtx layout.Context) {
	e.processSubmit(gtx)
	e.processCut(gtx)
	e.processCopy(gtx)
	e.processPaste(gtx)
}

func (e *MessageEditor) operationBarNeeded(gtx layout.Context) bool {
	return e.longPressed && gtx.Focused(&e.Editor) || e.Editor.SelectionLen() > 0
}

func (e *MessageEditor) processCut(gtx layout.Context) {
	if e.cutButton.Clicked(gtx) {
		if e.Editor.SelectionLen() > 0 {
			textForCopy := e.Editor.SelectedText()
			gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(textForCopy))})
			e.Editor.Delete(1)
		}
		e.hideOperationBar()
	}
}

func (e *MessageEditor) processPaste(gtx layout.Context) {
	if e.pasteButton.Clicked(gtx) {
		if e.Editor.SelectionLen() > 0 {
			e.Editor.Delete(1)
		}
		gtx.Execute(clipboard.ReadCmd{Tag: &e.Editor})
		e.hideOperationBar()
	}
}

func (e *MessageEditor) processCopy(gtx layout.Context) {
	if e.copyButton.Clicked(gtx) {
		textForCopy := e.Editor.Text()
		if e.Editor.SelectionLen() > 0 {
			textForCopy = e.Editor.SelectedText()
		}
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(textForCopy))})
		e.hideOperationBar()
	}
}

func (e *MessageEditor) hideOperationBar() {
	e.longPressed = false
	if e.Editor.SelectionLen() > 0 {
		e.Editor.ClearSelection()
	}
}
func (e *MessageEditor) drawSubmitButton(gtx layout.Context) layout.Dimensions {
	return material.IconButtonStyle{
		Background: e.Theme.ContrastBg,
		Color:      e.Theme.ContrastFg,
		Icon:       icons.SubmitIcon,
		Size:       unit.Dp(24.0),
		Button:     &e.submitButton,
		Inset:      layout.UniformInset(unit.Dp(9)),
	}.Layout(gtx)
}

func (e *MessageEditor) Submitted(gtx layout.Context) bool {
	return e.submitButton.Clicked(gtx) || e.submittedByCarriageReturn(gtx)
}

func (e *MessageEditor) submittedByCarriageReturn(gtx layout.Context) (submit bool) {
	for {
		ev, ok := e.Editor.Update(gtx)
		if _, submit = ev.(widget.SubmitEvent); submit {
			break
		}
		if !ok {
			break
		}
	}
	return submit
}

func (e *MessageEditor) processSubmit(gtx layout.Context) {
	// ---------- Handle input ----------
	if e.Submitted(gtx) {
		msg := strings.TrimSpace(e.Editor.Text())
		e.Editor.SetText("")
		if msg == "" {
			return
		}
		go func() {
			message := NewTextMessage(msg)
			MessageBox <- message
			if wi.DefaultClient.SendText(msg) == nil {
				message.State = Sent
			} else {
				message.State = Failed
			}
		}()
	}
}

func NewTextMessage(msg string) *Message {
	return &Message{State: Stateless,
		TextControl: NewTextControl(msg),
		MessageStyle: MessageStyle{
			Theme: fonts.DefaultTheme,
		},
		Contacts:    FromMyself(),
		MessageType: Text,
		CreatedAt:   time.Now()}
}

func NewInvisibleMessage() *Message {
	return NewTextMessage("")
}

type EditorOperator struct {
	cutButton   widget.Clickable
	copyButton  widget.Clickable
	pasteButton widget.Clickable
}

func (e *EditorOperator) Layout(gtx layout.Context) {
	centerOffset, iconSize, margin := 54, 28, 8
	defer op.Offset(image.Point{Y: -gtx.Dp(unit.Dp(iconSize + 24))}).Push(gtx.Ops).Pop()
	macro := op.Record(gtx.Ops)
	icons := layout.UniformInset(unit.Dp(margin)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				offset := image.Pt(-gtx.Dp(unit.Dp(centerOffset+iconSize+margin)), 0)
				op.Offset(offset).Add(gtx.Ops)
				return e.cutButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return icons.ContentCutIcon.Layout(gtx, fonts.DefaultTheme.ContrastFg)
				})
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				offset := image.Pt(-gtx.Dp(unit.Dp(centerOffset)), 0)
				op.Offset(offset).Add(gtx.Ops)
				return e.copyButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return icons.ContentCopyIcon.Layout(gtx, fonts.DefaultTheme.ContrastFg)
				})
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				offset := image.Pt(-gtx.Dp(unit.Dp(centerOffset-iconSize-margin)), 0)
				op.Offset(offset).Add(gtx.Ops)
				return e.pasteButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return icons.ContentPasteIcon.Layout(gtx, fonts.DefaultTheme.ContrastFg)
				})
			}),
		)
	})
	call := macro.Stop()
	e.drawBorder(gtx, icons, call)
}

func (e *EditorOperator) drawBorder(gtx layout.Context, icons layout.Dimensions, call op.CallOp) {
	centerOffset, iconSize, margin := gtx.Dp(54), gtx.Dp(28), gtx.Dp(8)
	bgColor := fonts.DefaultTheme.ContrastBg
	bgColor.A = 220

	// Draw shadow first
	shadowColor := color.NRGBA{R: 0, G: 0, B: 0, A: 60}
	shadowRadius := gtx.Dp(8)
	e.drawShadowPath(gtx, icons, shadowColor, shadowRadius)

	// https://pomax.github.io/bezierinfo/#circles_cubic.
	const q = 4 * (math.Sqrt2 - 1) / 3
	const iq = 1 - q
	midX := float32(icons.Size.X)/2 - float32(centerOffset)
	minX := midX - float32(iconSize)*float32(1.5) - float32(margin)*2
	maxX := midX + float32(iconSize)*float32(1.5) + float32(margin)*2
	minY := float32(0)
	maxY := float32(iconSize) + float32(margin)*2
	se, sw, nw, ne := float32(margin), float32(margin), float32(margin), float32(margin)
	triangleLegHalfSize := float32(gtx.Dp(4))
	perpendicular := float32(float64(triangleLegHalfSize*2) * math.Sin(math.Pi/3))

	p := clip.Path{}
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Point{X: minX + nw, Y: minY})
	p.LineTo(f32.Point{X: maxX - ne, Y: minY})    // N
	p.CubeTo(f32.Point{X: maxX - ne*iq, Y: minY}, // NE
		f32.Point{X: maxX, Y: minY + ne*iq},
		f32.Point{X: maxX, Y: minY + ne})
	p.LineTo(f32.Point{X: maxX, Y: maxY - se})    // E
	p.CubeTo(f32.Point{X: maxX, Y: maxY - se*iq}, // SE
		f32.Point{X: maxX - se*iq, Y: maxY},
		f32.Point{X: maxX - se, Y: maxY})
	p.LineTo(f32.Point{X: midX + triangleLegHalfSize, Y: maxY}) // S
	p.LineTo(f32.Point{X: midX, Y: maxY + perpendicular})       // S
	p.LineTo(f32.Point{X: midX - triangleLegHalfSize, Y: maxY}) // S
	p.LineTo(f32.Point{X: minX + sw, Y: maxY})                  // S
	p.CubeTo(f32.Point{X: minX + sw*iq, Y: maxY},               // SW
		f32.Point{X: minX, Y: maxY - sw*iq},
		f32.Point{X: minX, Y: maxY - sw})
	p.LineTo(f32.Point{X: minX, Y: minY + nw})    // W
	p.CubeTo(f32.Point{X: minX, Y: minY + nw*iq}, // NW
		f32.Point{X: minX + nw*iq, Y: minY},
		f32.Point{X: minX + nw, Y: minY})
	path := p.End()

	defer clip.Outline{Path: path}.Op().Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, bgColor)
	pointer.CursorPointer.Add(gtx.Ops)
	defer pointer.StopOp{}.Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
}

func (e *EditorOperator) drawShadowPath(gtx layout.Context, icons layout.Dimensions, shadowColor color.NRGBA, shadowRadius int) {
	margin := gtx.Dp(8)
	iconSize := gtx.Dp(28)
	centerOffset := gtx.Dp(54)

	const q = 4 * (math.Sqrt2 - 1) / 3
	const iq = 1 - q
	midX := float32(icons.Size.X)/2 - float32(centerOffset)
	minX := midX - float32(iconSize)*float32(1.5) - float32(margin)*2
	maxX := midX + float32(iconSize)*float32(1.5) + float32(margin)*2
	minY := float32(shadowRadius)
	maxY := float32(iconSize) + float32(margin)*2 + float32(shadowRadius)
	se, sw, nw, ne := float32(margin)+float32(shadowRadius), float32(margin)+float32(shadowRadius), float32(margin)+float32(shadowRadius), float32(margin)+float32(shadowRadius)
	triangleLegHalfSize := float32(gtx.Dp(4))
	perpendicular := float32(float64(triangleLegHalfSize*2) * math.Sin(math.Pi/3))

	p := clip.Path{}
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Point{X: minX + nw, Y: minY})
	p.LineTo(f32.Point{X: maxX - ne, Y: minY})
	p.CubeTo(f32.Point{X: maxX - ne*iq, Y: minY},
		f32.Point{X: maxX, Y: minY + ne*iq},
		f32.Point{X: maxX, Y: minY + ne})
	p.LineTo(f32.Point{X: maxX, Y: maxY - se})
	p.CubeTo(f32.Point{X: maxX, Y: maxY - se*iq},
		f32.Point{X: maxX - se*iq, Y: maxY},
		f32.Point{X: maxX - se, Y: maxY})
	p.LineTo(f32.Point{X: midX + triangleLegHalfSize, Y: maxY})
	p.LineTo(f32.Point{X: midX, Y: maxY + perpendicular})
	p.LineTo(f32.Point{X: midX - triangleLegHalfSize, Y: maxY})
	p.LineTo(f32.Point{X: minX + sw, Y: maxY})
	p.CubeTo(f32.Point{X: minX + sw*iq, Y: maxY},
		f32.Point{X: minX, Y: maxY - sw*iq},
		f32.Point{X: minX, Y: maxY - sw})
	p.LineTo(f32.Point{X: minX, Y: minY + nw})
	p.CubeTo(f32.Point{X: minX, Y: minY + nw*iq},
		f32.Point{X: minX + nw*iq, Y: minY},
		f32.Point{X: minX + nw, Y: minY})
	path := p.End()

	defer clip.Outline{Path: path}.Op().Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, shadowColor)
}
