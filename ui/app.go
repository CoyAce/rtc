package ui

import (
	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

type C = layout.Context
type D = layout.Dimensions

func Draw(window *app.Window) error {
	// theme defines the material design style
	theme := material.NewTheme()
	// ops are the operations from the UI
	var ops op.Ops

	// Define a tag for input routing
	var msgTag = "msgTag"
	msgs := []string{"hello", "world", "hello beautiful world"}
	for i := 1; i <= 20; i++ {
		msgs = append(msgs, "dummy message")
	}

	// y-position for text
	var scrollY unit.Dp = 0
	// submitButton is a clickable widget
	var submitButton widget.Clickable
	var expandButton widget.Clickable
	inputField := component.TextField{Editor: widget.Editor{Submit: true}}
	// sendMessage button icon
	iconSendMessage, _ := widget.NewIcon(icons.ContentSend)
	iconExpand, _ := widget.NewIcon(icons.NavigationUnfoldMore)
	// listen for events in the window.
	for {
		// detect what type of event
		switch e := window.Event().(type) {
		// this is sent when the application is closed
		case app.DestroyEvent:
			return e.Err

		// this is sent when the application should re-render.
		case app.FrameEvent:
			// This graphics context is used for managing the rendering state.
			gtx := app.NewContext(&ops, e)

			// ---------- Handle input ----------
			// Time to deal with inputs since last frame.

			// Scrolled a mouse wheel?
			for {
				ev, ok := gtx.Event(
					pointer.Filter{
						Target:  msgTag,
						Kinds:   pointer.Scroll,
						ScrollY: pointer.ScrollRange{Min: -1, Max: +1},
					},
				)
				if !ok {
					break
				}
				//fmt.Printf("SCROLL: %+v\n", ev)
				scrollY = scrollY + unit.Dp(ev.(pointer.Event).Scroll.Y*float32(theme.TextSize))*2
				if scrollY < 0 {
					scrollY = 0
				}
			}

			flex := layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}
			flex.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					// Then we use scrollY to control the distance from the top of the screen to the first element.
					// We visualize the text using a list where each paragraph is a separate item.
					var vizList = layout.List{
						Axis: layout.Vertical,
						Position: layout.Position{
							Offset: int(scrollY),
						},
					}
					dimensions := vizList.Layout(gtx, len(msgs), func(gtx C, index int) D {
						return Layout(gtx, msgs[index], theme)
					})
					// ---------- REGISTERING EVENTS ----------
					event.Op(&ops, msgTag)
					return dimensions
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					// Render with flexbox layout:
					return layout.Flex{
						// Vertical alignment, from top to bottom
						Axis: layout.Vertical,
						// Empty space is left at the start, i.e. at the top
						Spacing: layout.SpaceStart,
					}.Layout(gtx,
						// Rigid to hold message input field and submit button
						layout.Rigid(func(gtx C) D {
							// Define margins around the flex item using layout.Inset
							margins := layout.Inset{Left: unit.Dp(8.0), Right: unit.Dp(8)}
							return margins.Layout(gtx, func(gtx C) D {
								return layout.Flex{
									Axis:      layout.Horizontal,
									Spacing:   layout.SpaceBetween,
									Alignment: layout.End,
								}.Layout(gtx,
									// text input
									layout.Flexed(1.0, func(gtx C) D {
										return inputField.Layout(gtx, theme, "Message")
									}),
									// submit button
									layout.Rigid(func(gtx C) D {
										margins := layout.Inset{Left: unit.Dp(8.0)}
										return margins.Layout(gtx,
											func(gtx C) D {
												return material.IconButtonStyle{
													Background: theme.ContrastBg,
													Color:      theme.ContrastFg,
													Icon:       iconSendMessage,
													Size:       unit.Dp(24.0),
													Button:     &submitButton,
													Inset:      layout.UniformInset(unit.Dp(9)),
												}.Layout(gtx)
											},
										)
									}),
									// expand button
									layout.Rigid(func(gtx C) D {
										margins := layout.Inset{Left: unit.Dp(8.0)}
										return margins.Layout(
											gtx,
											func(gtx C) D {
												return material.IconButtonStyle{
													Background: theme.ContrastBg,
													Color:      theme.ContrastFg,
													Icon:       iconExpand,
													Size:       unit.Dp(24.0),
													Button:     &expandButton,
													Inset:      layout.UniformInset(unit.Dp(9)),
												}.Layout(gtx)
											},
										)
									}),
								)
							})
						}),
						// ... then one to hold an empty spacer
						layout.Rigid(
							// The height of the spacer is 25 Device independent pixels
							layout.Spacer{Height: unit.Dp(25)}.Layout,
						),
					)
				}),
			)

			// Pass the drawing operations to the GPU.
			e.Frame(gtx.Ops)
		}
	}

}
