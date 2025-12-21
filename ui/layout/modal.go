package layout

import (
	"image"
	"image/color"
	"time"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"
	"gioui.org/x/component"
)

type Modal interface {
	Show(widget layout.Widget, onBackdropClick func(), animation component.VisibilityAnimation)
	Dismiss(afterDismiss func())
	Layout(gtx layout.Context) layout.Dimensions
}
type appModal struct {
	onBackdropClick func()
	widget          layout.Widget
	backdropButton  widget.Clickable
	innerButton     widget.Clickable
	Animation       component.VisibilityAnimation
	afterDismiss    func()
}

type modalStack struct {
	Modals []*appModal
}

// Show add widget to modalStack
// onBackdropClick called when click outside of modal
// default behavior is close modal
func (s *modalStack) Show(widget layout.Widget, onBackdropClick func(), animation component.VisibilityAnimation) {
	modal := appModal{
		onBackdropClick: onBackdropClick,
		widget:          widget,
		Animation:       animation,
	}
	if onBackdropClick == nil {
		modal.onBackdropClick = func() {
			modal.Animation.Disappear(time.Now())
		}
	}
	s.Modals = append(s.Modals, &modal)
	modal.Show(widget)
}

func (s *modalStack) Dismiss(afterDismiss func()) {
	stackSize := len(s.Modals)
	if stackSize > 0 {
		s.Modals[stackSize-1].Dismiss(afterDismiss)
	}
}

func (s *modalStack) Layout(gtx layout.Context) layout.Dimensions {
	for _, modal := range s.Modals {
		modal.Layout(gtx)
	}
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func NewModalStack() Modal {
	m := &modalStack{}
	m.Modals = make([]*appModal, 0)
	return m
}

func (m *appModal) Show(widget layout.Widget) {
	m.widget = widget
	m.Animation.Appear(time.Now())
}

func (m *appModal) Layout(gtx layout.Context) layout.Dimensions {
	if m.backdropButton.Clicked(gtx) {
		m.onBackdropClick()
	}
	var finalPosY int
	return layout.Stack{Alignment: layout.N}.Layout(gtx,
		// backdrop button
		layout.Stacked(m.drawBackdropButton),
		// widget area
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			state := m.Animation.State
			progress := m.Animation.Revealed(gtx)
			// invisible case
			switch {
			case state == component.Invisible, progress == 0, m.widget == nil:
				if m.afterDismiss != nil && !m.Animation.Animating() {
					m.afterDismiss()
					m.afterDismiss = nil
				}
				return layout.Dimensions{}
			}
			// record Widget's dimension
			macro := op.Record(gtx.Ops)
			d := m.innerButton.Layout(gtx, m.widget)
			call := macro.Stop()

			// float down animation
			finalPosY = -d.Size.Y + int(float32((gtx.Constraints.Max.Y+d.Size.Y)/2)*progress)
			op.Offset(image.Point{
				X: 0,
				Y: finalPosY,
			}).Add(gtx.Ops)
			call.Add(gtx.Ops)
			return d
		}),
	)
}

func (m *appModal) drawBackdropButton(gtx layout.Context) layout.Dimensions {
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	return m.backdropButton.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			if m.Animation.Revealed(gtx) == 0 || m.widget == nil {
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}
			return component.Rect{Size: gtx.Constraints.Max, Color: color.NRGBA{A: 200}}.Layout(gtx)
		},
	)
}

func (m *appModal) Dismiss(afterDismiss func()) {
	m.Animation.Disappear(time.Now())
	m.afterDismiss = afterDismiss
}

var DefaultModal = NewModalStack()
