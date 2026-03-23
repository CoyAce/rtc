package view

import (
	"image"
	"image/color"
	"math"
	"mushin/assets/fonts"
	"mushin/assets/icons"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"golang.org/x/exp/shiny/materialdesign/colornames"
)

type Mode uint8

const (
	None Mode = iota
	Accept
	Decline
)

type IconButton struct {
	*material.Theme
	Icon    *widget.Icon
	Enabled bool
	Hidden  bool
	Mode
	OnClick func()
	button  widget.Clickable
}

func (b *IconButton) Layout(gtx layout.Context) layout.Dimensions {
	if b.button.Clicked(gtx) && b.OnClick != nil {
		b.OnClick()
	}
	bg := b.Theme.ContrastBg
	if !b.Enabled {
		bg = color.NRGBA(colornames.Grey500)
	}
	switch b.Mode {
	case Accept:
		bg = color.NRGBA(colornames.Green400)
	case Decline:
		bg = color.NRGBA(colornames.Red400)
	default:
	}
	return material.IconButtonStyle{
		Background: bg,
		Color:      b.Theme.ContrastFg,
		Icon:       b.Icon,
		Size:       unit.Dp(24.0),
		Button:     &b.button,
		Inset:      layout.UniformInset(unit.Dp(9)),
	}.Layout(gtx)
}

type IconStack struct {
	*material.Theme
	*component.VisibilityAnimation
	Sticky      bool
	IconButtons []*IconButton
}

func (s *IconStack) drawIconStackItemsWithGlitch(gtx layout.Context, progress float32) layout.Dimensions {
	elapsed := float32(time.Now().UnixNano()%1e9) / 1e9

	// IconButton size is fixed: 24dp icon + 9dp inset * 2 = 42dp
	buttonSize := gtx.Dp(42)
	spacing := gtx.Dp(8)

	// Calculate total height needed
	totalHeight := 0
	for i, button := range s.IconButtons {
		if button.Hidden {
			continue
		}

		buttonDelay := float32(i) * 0.08
		buttonProgress := float32(math.Max(0, float64((progress-buttonDelay)*5)))
		if buttonProgress > 1 {
			buttonProgress = 1
		}

		if buttonProgress > 0 {
			// Each button takes buttonSize + spacing between buttons
			totalHeight += buttonSize + spacing
		}
	}

	// Draw buttons from bottom to top (reverse order), starting with spacing offset
	yOffset := spacing
	for i := len(s.IconButtons) - 1; i >= 0; i-- {
		button := s.IconButtons[i]
		if button.Hidden {
			continue
		}

		buttonDelay := float32(i) * 0.08
		buttonProgress := float32(math.Max(0, float64((progress-buttonDelay)*5)))
		if buttonProgress > 1 {
			buttonProgress = 1
		}

		if buttonProgress > 0 {
			// Only apply glitch effect during animation (not when fully revealed)
			useGlitch := buttonProgress < 1.0

			var offsetX, offsetY int
			if useGlitch {
				// Apply individual glitch offset per button
				glitchPhase := float32(i) * 0.5
				offsetX = int(float32(gtx.Dp(3)) * float32(math.Sin(float64(elapsed*20+glitchPhase))))
				offsetY = int(float32(gtx.Dp(2)) * float32(math.Cos(float64(elapsed*15+glitchPhase))))
			}

			// Calculate button-specific glitch intensity
			buttonGlitchIntensity := float32(math.Max(0, float64((1-progress)*12))) * (1 - buttonProgress)
			buttonPhase := float32(i) * 1.2

			// Draw RGB split glitch layers for this button
			if buttonGlitchIntensity > 0.1 {
				// Calculate button position from top (same as main button)
				drawY := totalHeight - yOffset - buttonSize

				layerConfigs := []struct {
					color   color.NRGBA
					offsetX int
					offsetY int
				}{
					{
						// Red layer - aggressive offset
						color:   color.NRGBA{R: 255, G: 50, B: 100, A: uint8(150 * buttonGlitchIntensity)},
						offsetX: int(float32(gtx.Dp(6)) * float32(math.Sin(float64(elapsed*25+buttonPhase)))),
						offsetY: int(float32(gtx.Dp(4)) * float32(math.Cos(float64(elapsed*18+buttonPhase)))),
					},
					{
						// Green layer - wild movement
						color:   color.NRGBA{R: 50, G: 255, B: 150, A: uint8(140 * buttonGlitchIntensity)},
						offsetX: int(float32(gtx.Dp(5)) * float32(math.Cos(float64(elapsed*22+buttonPhase)))),
						offsetY: int(float32(gtx.Dp(6)) * float32(math.Sin(float64(elapsed*15+buttonPhase)))),
					},
					{
						// Blue layer - maximum displacement
						color:   color.NRGBA{R: 50, G: 100, B: 255, A: uint8(160 * buttonGlitchIntensity)},
						offsetX: int(float32(gtx.Dp(8)) * float32(math.Sin(float64(elapsed*30+buttonPhase)))),
						offsetY: int(float32(gtx.Dp(3)) * float32(math.Cos(float64(elapsed*28+buttonPhase)))),
					},
				}

				for _, layer := range layerConfigs {
					if layer.color.A > 20 {
						// Use drawY for consistent positioning with main button
						layerOffset := image.Pt(offsetX+layer.offsetX, drawY+offsetY+layer.offsetY)
						op.Offset(layerOffset).Add(gtx.Ops)

						// Draw semi-transparent glitch layer - constrain to button bounds only
						macro := op.Record(gtx.Ops)
						button.Layout(gtx)
						call := macro.Stop()

						// Apply color tint only within button bounds
						paint.FillShape(gtx.Ops, layer.color, clip.Rect{
							Min: image.Point{X: offsetX, Y: drawY + offsetY},
							Max: image.Point{X: offsetX + buttonSize, Y: drawY + offsetY + buttonSize},
						}.Op())
						call.Add(gtx.Ops)

						// Reset offset
						op.Offset(image.Pt(-layerOffset.X, -layerOffset.Y)).Add(gtx.Ops)
					}
				}
			}

			// Draw main button
			// Position from bottom: totalHeight - yOffset (with extra spacing at bottom)
			drawY := totalHeight - yOffset - buttonSize
			op.Offset(image.Pt(offsetX, drawY+offsetY)).Add(gtx.Ops)

			// Draw button with fixed size constraint (square button)
			drawGtx := gtx
			drawGtx.Constraints.Min.X = buttonSize
			drawGtx.Constraints.Max.X = buttonSize
			drawGtx.Constraints.Min.Y = buttonSize
			drawGtx.Constraints.Max.Y = buttonSize
			button.Layout(drawGtx)

			// Restore position
			op.Offset(image.Pt(-offsetX, -drawY-offsetY)).Add(gtx.Ops)

			yOffset += buttonSize + spacing
		}
	}

	return layout.Dimensions{Size: image.Point{X: buttonSize, Y: totalHeight}}
}

func (s *IconStack) Layout(gtx layout.Context) (layout.Dimensions, layout.Dimensions) {
	s.update(gtx)
	var d layout.Dimensions
	return layout.Stack{Alignment: layout.SE}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			offset := image.Pt(-gtx.Dp(8), -gtx.Dp(57))
			op.Offset(offset).Add(gtx.Ops)
			progress := s.VisibilityAnimation.Revealed(gtx)

			// No separate glitch layers - they're integrated into button drawing

			macro := op.Record(gtx.Ops)
			d = s.drawIconStackItemsWithGlitch(gtx, progress)
			call := macro.Stop()

			// No need to scale by progress - buttons animate individually
			// d.Size.Y = int(float32(d.Size.Y) * progress)

			component.Rect{Size: d.Size, Color: color.NRGBA{}}.Layout(gtx)
			defer clip.Rect{Max: d.Size}.Push(gtx.Ops).Pop()
			event.Op(gtx.Ops, s)
			call.Add(gtx.Ops)
			return d
		}),
	), d
}

func (s *IconStack) update(gtx layout.Context) {
	if !s.Sticky && s.State == component.Visible && !gtx.Focused(s) {
		gtx.Execute(key.FocusCmd{Tag: s})
		gtx.Execute(op.InvalidateCmd{})
	}
	for {
		e, ok := gtx.Event(
			key.FocusFilter{Target: s},
		)
		if !ok {
			break
		}
		switch e := e.(type) {
		case key.FocusEvent:
			if !e.Focus && !s.Sticky {
				s.VisibilityAnimation.Disappear(gtx.Now)
			}
		}
	}
}

var iconStackAnimation = component.VisibilityAnimation{
	Duration: time.Millisecond * 800, // Match staggered button animation (6 buttons × 80ms + buffer)
	State:    component.Invisible,
	Started:  time.Time{},
}

func NewIconStack(modeSwitch func(*IconButton) func(), appendFile func(mapping *FileDescription)) *IconStack {
	settings := NewSettingsForm(OnSettingsSubmit)
	audioMakeButton.OnClick = MakeAudioCall(audioMakeButton)
	voiceMessageSwitch := &IconButton{Theme: fonts.DefaultTheme, Icon: icons.VoiceMessageIcon, Enabled: true}
	voiceMessageSwitch.OnClick = modeSwitch(voiceMessageSwitch)
	return &IconStack{
		Sticky:              false,
		Theme:               fonts.DefaultTheme,
		VisibilityAnimation: &iconStackAnimation,
		IconButtons: []*IconButton{
			{Theme: fonts.DefaultTheme, Icon: icons.SettingsIcon, Enabled: true, OnClick: settings.ShowWithModal},
			{Theme: fonts.DefaultTheme, Icon: icons.FilesIcon, Enabled: true, OnClick: ChooseAndSendFile(appendFile)},
			{Theme: fonts.DefaultTheme, Icon: icons.PhotoLibraryIcon, Enabled: true, OnClick: ChooseAndSendPhoto},
			{Theme: fonts.DefaultTheme, Icon: icons.VideoCallIcon},
			audioMakeButton,
			voiceMessageSwitch,
		},
	}
}

type ExpandButton struct {
	expandButton   widget.Clickable
	collapseButton widget.Clickable
}

func (e *ExpandButton) Layout(gtx layout.Context) layout.Dimensions {
	margins := layout.Inset{Left: unit.Dp(8.0)}
	return margins.Layout(
		gtx,
		func(gtx layout.Context) layout.Dimensions {
			btn := &e.expandButton
			icon := icons.ExpandIcon
			if e.collapseButton.Clicked(gtx) {
				iconStackAnimation.Disappear(gtx.Now)
			}
			if e.expandButton.Clicked(gtx) {
				iconStackAnimation.Appear(gtx.Now)
			}
			if iconStackAnimation.Revealed(gtx) != 0 {
				btn = &e.collapseButton
				icon = icons.CollapseIcon
			}
			return material.IconButtonStyle{
				Background: fonts.DefaultTheme.ContrastBg,
				Color:      fonts.DefaultTheme.ContrastFg,
				Icon:       icon,
				Size:       unit.Dp(24.0),
				Button:     btn,
				Inset:      layout.UniformInset(unit.Dp(9)),
			}.Layout(gtx)
		},
	)
}
