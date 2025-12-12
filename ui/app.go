package ui

import (
	"math"
	"rtc/core"
	"strings"
	"time"

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

func Draw(window *app.Window, client core.Client) error {
	// theme defines the material design style
	theme := material.NewTheme()
	// ops are the operations from the UI
	var ops op.Ops

	// Define a tag for input routing
	var msgTag = "msgTag"
	var msgs []Message

	var scrollToEnd = false
	var firstVisible = false
	// y-position for msg list
	var scrollY unit.Dp = 0
	var maxOffset unit.Dp = 0
	// listen for events in the msgs channel
	go func() {
		for m := range client.SignedMessages {
			message := Message{State: Sent, UUID: client.UUID, Sender: m.UUID, Text: string(m.Payload), CreatedAt: time.Now()}
			msgs = append(msgs, message)
			scrollToEnd = true
			// update scroll offset
			maxOffset += unit.Dp(theme.TextSize) * 10
			scrollY = maxOffset
			window.Invalidate()
		}
	}()

	// submitButton is a clickable widget
	var submitButton widget.Clickable
	var expandButton widget.Clickable
	inputField := component.TextField{Editor: widget.Editor{Submit: true}}
	// icons
	submitIcon, _ := widget.NewIcon(icons.ContentSend)
	expandIcon, _ := widget.NewIcon(icons.NavigationUnfoldMore)
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
			maxOffset = unit.Dp(math.Max(float64(scrollY), float64(maxOffset)))
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
				scrollToEnd = false
				scrollY = scrollY + unit.Dp(ev.(pointer.Event).Scroll.Y*float32(theme.TextSize))*3
				if scrollY < 0 {
					scrollY = 0
				}
			}

			if submitButton.Clicked(gtx) || submittedByCarriageReturn(&inputField, gtx) {
				msg := strings.TrimSpace(inputField.Text())
				client.SendText(msg)
				inputField.Clear()
				if client.Disconnected {
					message := Message{Sender: client.UUID, UUID: client.UUID, Text: msg, CreatedAt: time.Now(), State: Stateless}
					msgs = append(msgs, message)
				}
			}

			flex := layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}
			flex.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					// Then we use scrollY to control the distance from the top of the screen to the first element.
					// We visualize the text using a list where each paragraph is a separate item.
					var vizList = layout.List{
						Axis: layout.Vertical,
						Position: layout.Position{
							Offset: int(scrollY),
						},
						ScrollToEnd: firstVisible || scrollToEnd,
					}
					dimensions := vizList.Layout(gtx, len(msgs), func(gtx layout.Context, index int) layout.Dimensions {
						return msgs[index].Layout(gtx, theme)
					})
					// at end of list
					if !vizList.Position.BeforeEnd {
						// scroll down invalid when at list end
						if scrollY > maxOffset {
							scrollY = maxOffset
						}
						// first item visible
						firstVisible = vizList.Position.First == 0
					}
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
						layout.Rigid(
							// The height of the spacer is 8 Device independent pixels
							layout.Spacer{Height: unit.Dp(8)}.Layout,
						),
						// Rigid to hold message input field and submit button
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							// Define margins around the flex item using layout.Inset
							margins := layout.Inset{Left: unit.Dp(8.0), Right: unit.Dp(8)}
							return margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{
									Axis:      layout.Horizontal,
									Spacing:   layout.SpaceBetween,
									Alignment: layout.End,
								}.Layout(gtx,
									// text input
									layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
										return inputField.Layout(gtx, theme, "Message")
									}),
									// submit button
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										margins := layout.Inset{Left: unit.Dp(8.0)}
										return margins.Layout(gtx,
											func(gtx layout.Context) layout.Dimensions {
												return material.IconButtonStyle{
													Background: theme.ContrastBg,
													Color:      theme.ContrastFg,
													Icon:       submitIcon,
													Size:       unit.Dp(24.0),
													Button:     &submitButton,
													Inset:      layout.UniformInset(unit.Dp(9)),
												}.Layout(gtx)
											},
										)
									}),
									// expand button
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										// disable expand
										return layout.Dimensions{}
										margins := layout.Inset{Left: unit.Dp(8.0)}
										return margins.Layout(
											gtx,
											func(gtx layout.Context) layout.Dimensions {
												return material.IconButtonStyle{
													Background: theme.ContrastBg,
													Color:      theme.ContrastFg,
													Icon:       expandIcon,
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
							// The height of the spacer is 15 Device independent pixels
							layout.Spacer{Height: unit.Dp(15)}.Layout,
						),
					)
				}),
			)

			// Pass the drawing operations to the GPU.
			e.Frame(gtx.Ops)
		}
	}
}

func submittedByCarriageReturn(editor *component.TextField, gtx layout.Context) (submit bool) {
	for {
		ev, ok := editor.Editor.Update(gtx)
		if _, submit = ev.(widget.SubmitEvent); submit {
			break
		}
		if !ok {
			break
		}
	}
	return submit
}
