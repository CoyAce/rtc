package view

import (
	"image"
	"io"
	"math"
	"rtc/assets/fonts"
	"rtc/assets/icons"
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
	"gioui.org/x/component"
	"github.com/CoyAce/wi"
)

type MessageEditor struct {
	*material.Theme
	InteractiveSpan
	EditorOperator
	ExpandButton
	InputField   *component.TextField
	submitButton widget.Clickable
}

func (e *MessageEditor) Layout(gtx layout.Context) layout.Dimensions {
	e.update(gtx)
	if e.operationBarNeeded(gtx) {
		e.EditorOperator.Layout(gtx)
	}
	if !gtx.Focused(&e.InputField.Editor) {
		e.hideOperationBar()
	}
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	e.InteractiveSpan.Layout(gtx)
	margins := layout.Inset{Top: unit.Dp(8.0), Left: unit.Dp(8.0), Right: unit.Dp(8), Bottom: unit.Dp(15)}
	return margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Spacing:   layout.SpaceBetween,
			Alignment: layout.End,
		}.Layout(gtx,
			// text input
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				return e.InputField.Layout(gtx, e.Theme, "Message")
			}),
			// submit button
			layout.Rigid(e.drawSubmitButton),
			// expand button
			layout.Rigid(e.ExpandButton.Layout),
		)
	})
}

func (e *MessageEditor) update(gtx layout.Context) {
	e.processSubmit(gtx)
	e.processCut(gtx)
	e.processCopy(gtx)
	e.processPaste(gtx)
}

func (e *MessageEditor) operationBarNeeded(gtx layout.Context) bool {
	return e.longPressed && gtx.Focused(&e.InputField.Editor) || e.InputField.SelectionLen() > 0
}

func (e *MessageEditor) processCut(gtx layout.Context) {
	if e.cutButton.Clicked(gtx) {
		if e.InputField.Editor.SelectionLen() > 0 {
			textForCopy := e.InputField.Editor.SelectedText()
			gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(textForCopy))})
			e.InputField.Editor.Delete(1)
		}
		e.hideOperationBar()
	}
}

func (e *MessageEditor) processPaste(gtx layout.Context) {
	if e.pasteButton.Clicked(gtx) {
		if e.InputField.Editor.SelectionLen() > 0 {
			e.InputField.Editor.Delete(1)
		}
		gtx.Execute(clipboard.ReadCmd{Tag: &e.InputField.Editor})
		e.hideOperationBar()
	}
}

func (e *MessageEditor) processCopy(gtx layout.Context) {
	if e.copyButton.Clicked(gtx) {
		textForCopy := e.InputField.Text()
		if e.InputField.Editor.SelectionLen() > 0 {
			textForCopy = e.InputField.Editor.SelectedText()
		}
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(textForCopy))})
		e.hideOperationBar()
	}
}

func (e *MessageEditor) hideOperationBar() {
	e.longPressed = false
	if e.InputField.Editor.SelectionLen() > 0 {
		e.InputField.Editor.ClearSelection()
	}
}

func (e *MessageEditor) drawSubmitButton(gtx layout.Context) layout.Dimensions {
	margins := layout.Inset{Left: unit.Dp(8.0)}
	return margins.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return material.IconButtonStyle{
				Background: e.Theme.ContrastBg,
				Color:      e.Theme.ContrastFg,
				Icon:       icons.SubmitIcon,
				Size:       unit.Dp(24.0),
				Button:     &e.submitButton,
				Inset:      layout.UniformInset(unit.Dp(9)),
			}.Layout(gtx)
		},
	)
}

func (e *MessageEditor) Submitted(gtx layout.Context) bool {
	return e.submitButton.Clicked(gtx) || e.submittedByCarriageReturn(gtx)
}

func (e *MessageEditor) submittedByCarriageReturn(gtx layout.Context) (submit bool) {
	for {
		ev, ok := e.InputField.Editor.Update(gtx)
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
		msg := strings.TrimSpace(e.InputField.Text())
		e.InputField.Clear()
		go func() {
			message := Message{State: Stateless,
				TextControl: NewTextControl(msg),
				MessageStyle: MessageStyle{
					Theme: fonts.DefaultTheme,
				},
				Contacts:    FromMyself(),
				MessageType: Text,
				CreatedAt:   time.Now()}
			MessageBox <- &message
			if wi.DefaultClient.SendText(msg) == nil {
				message.State = Sent
			} else {
				message.State = Failed
			}
		}()
	}
}

type EditorOperator struct {
	cutButton   widget.Clickable
	copyButton  widget.Clickable
	pasteButton widget.Clickable
}

func (e *EditorOperator) Layout(gtx layout.Context) {
	centerOffset, iconSize, margin := 54, 24, 4
	defer op.Offset(image.Point{Y: -gtx.Dp(unit.Dp(iconSize))}).Push(gtx.Ops).Pop()
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
	centerOffset, iconSize, margin := gtx.Dp(54), gtx.Dp(24), gtx.Dp(4)
	bgColor := fonts.DefaultTheme.ContrastBg
	bgColor.A = 192
	// https://pomax.github.io/bezierinfo/#circles_cubic.
	const q = 4 * (math.Sqrt2 - 1) / 3
	const iq = 1 - q
	midX := float32(icons.Size.X)/2 - float32(centerOffset)
	minX := midX - float32(iconSize)*float32(1.5) - float32(margin)*2
	maxX := midX + float32(iconSize)*float32(1.5) + float32(margin)*2
	minY := float32(0)
	maxY := float32(iconSize) + float32(margin)*2
	se, sw, nw, ne := float32(margin), float32(margin), float32(margin), float32(margin)
	triangleLegHalfSize := float32(gtx.Dp(3))
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
	call.Add(gtx.Ops)
}
