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
	"gioui.org/io/semantic"
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

func drawInk(gtx layout.Context, c widget.Press) {
	// duration is the number of seconds for the
	// completed animation: expand while fading in, then
	// out.
	const (
		expandDuration = float32(0.5)
		fadeDuration   = float32(0.9)
	)

	now := gtx.Now

	t := float32(now.Sub(c.Start).Seconds())

	end := c.End
	if end.IsZero() {
		// If the press hasn't ended, don't fade-out.
		end = now
	}

	endt := float32(end.Sub(c.Start).Seconds())

	// Compute the fade-in/out position in [0;1].
	var alphat float32
	{
		var haste float32
		if c.Cancelled {
			// If the press was cancelled before the inkwell
			// was fully faded in, fast forward the animation
			// to match the fade-out.
			if h := 0.5 - endt/fadeDuration; h > 0 {
				haste = h
			}
		}
		// Fade in.
		half1 := t/fadeDuration + haste
		if half1 > 0.5 {
			half1 = 0.5
		}

		// Fade out.
		half2 := float32(now.Sub(end).Seconds())
		half2 /= fadeDuration
		half2 += haste
		if half2 > 0.5 {
			// Too old.
			return
		}

		alphat = half1 + half2
	}

	// Compute the expand position in [0;1].
	sizet := t
	if c.Cancelled {
		// Freeze expansion of cancelled presses.
		sizet = endt
	}
	sizet /= expandDuration

	// Animate only ended presses, and presses that are fading in.
	if !c.End.IsZero() || sizet <= 1.0 {
		gtx.Execute(op.InvalidateCmd{})
	}

	if sizet > 1.0 {
		sizet = 1.0
	}

	if alphat > .5 {
		// Start fadeout after half the animation.
		alphat = 1.0 - alphat
	}
	// Twice the speed to attain fully faded in at 0.5.
	t2 := alphat * 2
	// Beziér ease-in curve.
	alphaBezier := t2 * t2 * (3.0 - 2.0*t2)
	sizeBezier := sizet * sizet * (3.0 - 2.0*sizet)
	size := gtx.Constraints.Min.X
	if h := gtx.Constraints.Min.Y; h > size {
		size = h
	}
	// Cover the entire constraints min rectangle and
	// apply curve values to size and color.
	size = int(float32(size) * 2 * float32(math.Sqrt(2)) * sizeBezier)
	alpha := 0.7 * alphaBezier
	const col = 0.8
	ba, bc := byte(alpha*0xff), byte(col*0xff)
	rgba := MulAlpha(color.NRGBA{A: 0xff, R: bc, G: bc, B: bc}, ba)
	ink := paint.ColorOp{Color: rgba}
	ink.Add(gtx.Ops)
	rr := size / 2
	defer op.Offset(c.Position.Add(image.Point{
		X: -rr,
		Y: -rr,
	})).Push(gtx.Ops).Pop()
	defer clip.UniformRRect(image.Rectangle{Max: image.Pt(size, size)}, rr).Push(gtx.Ops).Pop()
	paint.PaintOp{}.Add(gtx.Ops)
}

// MulAlpha applies the alpha to the color.
func MulAlpha(c color.NRGBA, alpha uint8) color.NRGBA {
	c.A = uint8(uint32(c.A) * uint32(alpha) / 0xFF)
	return c
}

// GlitchIconButtonStyle is a cyberpunk-style icon button with glitch effects
type GlitchIconButtonStyle struct {
	Background  color.NRGBA
	Color       color.NRGBA
	Icon        []byte // IconVG data for glitch rendering
	Size        unit.Dp
	Inset       layout.Inset
	Button      *widget.Clickable
	Description string
	// Animation state
	Progress    float32 // 0-1, animation progress
	Phase       float32 // unique phase per button
	ElapsedTime float32 // elapsed time for oscillation
}

func (b GlitchIconButtonStyle) Layout(gtx layout.Context) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := b.Button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		if d := b.Description; d != "" {
			semantic.DescriptionOp(b.Description).Add(gtx.Ops)
		}

		// Calculate glitch intensity based on progress
		// Keep some glitch effect even after animation completes for cyberpunk style
		glitchIntensity := float32(math.Max(0, float64((1-b.Progress)*12)))
		// Always use glitch for icon color layers, but reduce intensity after animation
		useGlitchLayers := b.Progress < 1.0 && glitchIntensity > 0.1
		useGlitchColor := !useGlitchLayers // Only apply static glitch when not animating

		return layout.Background{}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				rr := (gtx.Constraints.Min.X + gtx.Constraints.Min.Y) / 4
				defer clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Min}, rr).Push(gtx.Ops).Pop()
				background := b.Background
				switch {
				case !gtx.Enabled():
					background = Disabled(b.Background)
				case b.Button.Hovered() || gtx.Focused(b.Button):
					background = Hovered(b.Background)
				}
				paint.Fill(gtx.Ops, background)
				for _, c := range b.Button.History() {
					drawInk(gtx, c)
				}
				return layout.Dimensions{Size: gtx.Constraints.Min}
			},
			func(gtx layout.Context) layout.Dimensions {
				return b.Inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					size := gtx.Dp(b.Size)
					iconSize := size

					// Use glitch-rendered icon if Icon is provided
					if b.Icon != nil && useGlitchColor {
						// Render icon with ultra glitch effect
						glitchImg := renderGlitchIcon(b.Icon, b.Color, iconSize)
						if glitchImg != nil {
							paint.NewImageOp(glitchImg).Add(gtx.Ops)
							paint.PaintOp{}.Add(gtx.Ops)
							return layout.Dimensions{Size: image.Point{X: size, Y: size}}
						}
					}

					// Draw glitch layers if animating (RGB split with offset)
					if useGlitchLayers {
						buttonPhase := b.Phase
						elapsed := b.ElapsedTime

						// RGB split layers - render icon with different colors and offsets
						layerConfigs := []struct {
							color   color.NRGBA
							offsetX int
							offsetY int
						}{
							{
								color:   color.NRGBA{R: 255, G: 50, B: 100, A: uint8(150 * glitchIntensity)},
								offsetX: int(float32(gtx.Dp(6)) * float32(math.Sin(float64(elapsed*25+buttonPhase)))),
								offsetY: int(float32(gtx.Dp(4)) * float32(math.Cos(float64(elapsed*18+buttonPhase)))),
							},
							{
								color:   color.NRGBA{R: 50, G: 255, B: 150, A: uint8(140 * glitchIntensity)},
								offsetX: int(float32(gtx.Dp(5)) * float32(math.Cos(float64(elapsed*22+buttonPhase)))),
								offsetY: int(float32(gtx.Dp(6)) * float32(math.Sin(float64(elapsed*15+buttonPhase)))),
							},
							{
								color:   color.NRGBA{R: 50, G: 100, B: 255, A: uint8(160 * glitchIntensity)},
								offsetX: int(float32(gtx.Dp(8)) * float32(math.Sin(float64(elapsed*30+buttonPhase)))),
								offsetY: int(float32(gtx.Dp(3)) * float32(math.Cos(float64(elapsed*28+buttonPhase)))),
							},
						}

						for _, layer := range layerConfigs {
							if layer.color.A > 20 && b.Icon != nil {
								// Render icon with layer color using Icon
								glitchImg := renderGlitchIcon(b.Icon, layer.color, size)
								if glitchImg != nil {
									op.Offset(image.Pt(layer.offsetX, layer.offsetY)).Add(gtx.Ops)
									paint.NewImageOp(glitchImg).Add(gtx.Ops)
									paint.PaintOp{}.Add(gtx.Ops)
									op.Offset(image.Pt(-layer.offsetX, -layer.offsetY)).Add(gtx.Ops)
								}
							}
						}
					}
					return layout.Dimensions{
						Size: image.Point{X: size, Y: size},
					}
				})
			},
		)
	})
	c := m.Stop()
	bounds := image.Rectangle{Max: dims.Size}
	defer clip.Ellipse(bounds).Push(gtx.Ops).Pop()
	c.Add(gtx.Ops)
	return dims
}

// Disabled blends color towards the luminance and multiplies alpha.
// Blending towards luminance will desaturate the color.
// Multiplying alpha blends the color together more with the background.
func Disabled(c color.NRGBA) (d color.NRGBA) {
	const r = 80 // blend ratio
	lum := approxLuminance(c)
	d = mix(c, color.NRGBA{A: c.A, R: lum, G: lum, B: lum}, r)
	d = MulAlpha(d, 128+32)
	return
}

// Hovered blends dark colors towards white, and light colors towards
// black. It is approximate because it operates in non-linear sRGB space.
func Hovered(c color.NRGBA) (h color.NRGBA) {
	if c.A == 0 {
		// Provide a reasonable default for transparent widgets.
		return color.NRGBA{A: 0x44, R: 0x88, G: 0x88, B: 0x88}
	}
	const ratio = 0x20
	m := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: c.A}
	if approxLuminance(c) > 128 {
		m = color.NRGBA{A: c.A}
	}
	return mix(m, c, ratio)
}

// mix mixes c1 and c2 weighted by (1 - a/256) and a/256 respectively.
func mix(c1, c2 color.NRGBA, a uint8) color.NRGBA {
	ai := int(a)
	return color.NRGBA{
		R: byte((int(c1.R)*ai + int(c2.R)*(256-ai)) / 256),
		G: byte((int(c1.G)*ai + int(c2.G)*(256-ai)) / 256),
		B: byte((int(c1.B)*ai + int(c2.B)*(256-ai)) / 256),
		A: byte((int(c1.A)*ai + int(c2.A)*(256-ai)) / 256),
	}
}

// approxLuminance is a fast approximate version of RGBA.Luminance.
func approxLuminance(c color.NRGBA) byte {
	const (
		r = 13933 // 0.2126 * 256 * 256
		g = 46871 // 0.7152 * 256 * 256
		b = 4732  // 0.0722 * 256 * 256
		t = r + g + b
	)
	return byte((r*int(c.R) + g*int(c.G) + b*int(c.B)) / t)
}

type Mode uint8

const (
	None Mode = iota
	Accept
	Decline
)

type IconButton struct {
	*material.Theme
	Icon    []byte      // IconVG data for glitch rendering
	Color   color.NRGBA // Custom icon color for glitch effects
	Enabled bool
	Hidden  bool
	Mode
	OnClick func()
	button  widget.Clickable
}

func (b *IconButton) Layout(gtx layout.Context, progress float32, phase float32, elapsed float32) layout.Dimensions {
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

	// Use custom color if provided, otherwise use theme's ContrastFg
	iconColor := b.Color
	if iconColor == (color.NRGBA{}) {
		iconColor = b.Theme.ContrastFg
	}

	return GlitchIconButtonStyle{
		Background:  bg,
		Color:       iconColor,
		Icon:        b.Icon,
		Size:        unit.Dp(24.0),
		Button:      &b.button,
		Inset:       layout.UniformInset(unit.Dp(9)),
		Progress:    progress,
		Phase:       phase,
		ElapsedTime: elapsed,
	}.Layout(gtx)
}

type IconStack struct {
	*material.Theme
	*component.VisibilityAnimation
	Sticky      bool
	IconButtons []*IconButton
}

func (s *IconStack) drawIconStackItemsWithGlitch(gtx layout.Context, progress float32) layout.Dimensions {
	elapsed := float32(s.Duration/time.Millisecond) * progress

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

	// Draw horizontal scanlines for retro monitor effect (across entire control panel)
	if totalHeight > 0 {
		scanlineCount := 30
		scanlineSpacing := totalHeight / scanlineCount
		scanlineHeight := scanlineSpacing / 4

		for i := 0; i < scanlineCount; i++ {
			y := i * scanlineSpacing
			// Animate scanlines: slow downward scroll + subtle vibration
			baseOffset := int(float32(y)+elapsed*float32(totalHeight)*0.3)%(totalHeight+scanlineSpacing) - scanlineSpacing
			vibration := int(math.Sin(float64(elapsed*15+float32(i)*0.5)) * 1.5) // Subtle jitter
			offsetY := baseOffset + vibration

			// Alternating scanline colors (cyan and magenta)
			// Use minimum opacity during animation, fade out when fully expanded
			scanlineAlpha := uint8(100) // Base alpha for visibility
			if progress > 0.8 {
				// Fade out when almost fully expanded
				scanlineAlpha = uint8(100 * (1.0 - progress))
			}

			scanlineColor := color.NRGBA{}
			if i%2 == 0 {
				scanlineColor = color.NRGBA{R: 0, G: 255, B: 255, A: scanlineAlpha}
			} else {
				scanlineColor = color.NRGBA{R: 255, G: 0, B: 255, A: scanlineAlpha}
			}

			if scanlineColor.A > 20 {
				paint.FillShape(gtx.Ops, scanlineColor, clip.Rect{
					Min: image.Point{X: 0, Y: offsetY},
					Max: image.Point{X: buttonSize, Y: offsetY + scanlineHeight},
				}.Op())
			}
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
			// Apply individual glitch offset per button for positioning
			useGlitch := buttonProgress < 1.0
			var offsetX, offsetY int
			if useGlitch {
				glitchPhase := float32(i) * 0.5
				offsetX = int(float32(gtx.Dp(3)) * float32(math.Sin(float64(elapsed*20+glitchPhase))))
				offsetY = int(float32(gtx.Dp(2)) * float32(math.Cos(float64(elapsed*15+glitchPhase))))
			}

			// Calculate button position from top
			drawY := totalHeight - yOffset - buttonSize
			op.Offset(image.Pt(offsetX, drawY+offsetY)).Add(gtx.Ops)

			// Use GlitchIconButtonStyle which handles all glitch effects internally
			buttonPhase := float32(i) * 1.2
			button.Layout(gtx, buttonProgress, buttonPhase, elapsed)

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
	voiceMessageSwitch := &IconButton{Theme: fonts.DefaultTheme, Icon: icons.AVMic, Enabled: true}
	voiceMessageSwitch.OnClick = modeSwitch(voiceMessageSwitch)

	// Macaron-inspired cyberpunk color palette for glitch effects
	settingsColor := color.NRGBA{R: 100, G: 181, B: 246, A: 255} // Sky Blue - tech & wisdom (Settings)
	audioColor := color.NRGBA{R: 255, G: 138, B: 101, A: 255}    // Coral Orange - energy & communication (Audio Call)
	voiceColor := color.NRGBA{R: 206, G: 147, B: 216, A: 255}    // Lavender Pink - creativity & expression (Voice Message)
	filesColor := color.NRGBA{R: 165, G: 214, B: 167, A: 255}    // Sage Green - organization & growth (Files)
	photoColor := color.NRGBA{R: 255, G: 183, B: 77, A: 255}     // Amber Yellow - creativity & memories (Photos)
	videoColor := color.NRGBA{R: 171, G: 183, B: 183, A: 255}    // Cool Gray - connection & professionalism (Video Call)

	// Create buttons with custom colors
	settingsButton := &IconButton{Theme: fonts.DefaultTheme, Icon: icons.ActionSettings, Enabled: true, OnClick: settings.ShowWithModal, Color: settingsColor}
	filesButton := &IconButton{Theme: fonts.DefaultTheme, Icon: icons.FileFolder, Enabled: true, OnClick: ChooseAndSendFile(appendFile), Color: filesColor}
	photoButton := &IconButton{Theme: fonts.DefaultTheme, Icon: icons.ImagePhotoLibrary, Enabled: true, OnClick: ChooseAndSendPhoto, Color: photoColor}
	videoButton := &IconButton{Theme: fonts.DefaultTheme, Icon: icons.AVVideoCall, Color: videoColor}
	audioMakeButton.Color = audioColor
	voiceMessageSwitch.Color = voiceColor

	return &IconStack{
		Sticky:              false,
		Theme:               fonts.DefaultTheme,
		VisibilityAnimation: &iconStackAnimation,
		IconButtons: []*IconButton{
			settingsButton,
			filesButton,
			photoButton,
			videoButton,
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
	btn := &e.expandButton
	if e.collapseButton.Clicked(gtx) {
		iconStackAnimation.Disappear(gtx.Now)
	}
	if e.expandButton.Clicked(gtx) {
		iconStackAnimation.Appear(gtx.Now)
	}
	progress := iconStackAnimation.Revealed(gtx)
	if progress != 0 {
		btn = &e.collapseButton
	}
	return GlitchIconButtonStyle{
		Background:  fonts.DefaultTheme.Bg,
		Color:       fonts.DefaultTheme.ContrastFg,
		Icon:        icons.Circle,
		Size:        unit.Dp(24.0),
		Button:      btn,
		Inset:       layout.UniformInset(unit.Dp(9)),
		Progress:    progress,
		ElapsedTime: float32(iconStackAnimation.Duration/time.Millisecond) * progress,
	}.Layout(gtx)
}
