package ui

import (
	"rtc/assets/fonts"
	"rtc/core"
	"strings"
	"time"

	"gioui.org/app"
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
	theme := fonts.NewTheme()
	// ops are the operations from the UI
	var ops op.Ops

	var animation = component.VisibilityAnimation{
		Duration: time.Millisecond * 250,
		State:    component.Invisible,
		Started:  time.Time{},
	}

	//var messages []Message
	var messageList = &MessageList{List: layout.List{Axis: layout.Vertical}, Theme: theme}
	//var scrollToEnd, firstVisible = false, false
	// listen for events in the messages channel
	go func() {
		for m := range client.SignedMessages {
			message := Message{Theme: theme, State: Sent, UUID: client.UUID, Sender: m.UUID, Text: string(m.Payload), CreatedAt: time.Now()}
			message.AddTo(messageList)
			messageList.ScrollToEnd = true
			window.Invalidate()
		}
	}()

	// messageList
	// submitButton is a clickable widget
	var submitButton widget.Clickable
	var expandButton widget.Clickable
	var collapseButton widget.Clickable
	inputField := component.TextField{Editor: widget.Editor{Submit: true}}
	// icons
	submitIcon, _ := widget.NewIcon(icons.ContentSend)
	expandIcon, _ := widget.NewIcon(icons.NavigationUnfoldMore)
	collapseIcon, _ := widget.NewIcon(icons.NavigationUnfoldLess)
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
			if submitButton.Clicked(gtx) || submittedByCarriageReturn(&inputField, gtx) {
				msg := strings.TrimSpace(inputField.Text())
				if client.SendText(msg) != nil || client.Disconnected {
					message := Message{Theme: theme, Sender: client.UUID, UUID: client.UUID, Text: msg, CreatedAt: time.Now(), State: Stateless}
					message.AddTo(messageList)
					messageList.ScrollToEnd = true
				}
				inputField.Clear()
			}

			flex := layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}
			flex.Layout(gtx,
				layout.Flexed(1, messageList.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					// Render with flexbox layout:
					return layout.Flex{
						// Vertical alignment, from top to bottom
						Axis: layout.Vertical,
						// Empty space is left at the start, i.e. at the top
						Spacing: layout.SpaceStart,
					}.Layout(gtx,
						// Rigid to hold message input field and submit button
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							// Define margins around the flex item using layout.Inset
							margins := layout.Inset{Top: unit.Dp(8.0), Left: unit.Dp(8.0), Right: unit.Dp(8), Bottom: unit.Dp(15)}
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
												btn := &expandButton
												icon := expandIcon
												if collapseButton.Clicked(gtx) {
													animation.Disappear(gtx.Now)
												}
												if expandButton.Clicked(gtx) {
													animation.Appear(gtx.Now)
												}
												if animation.Revealed(gtx) != 0 {
													btn = &collapseButton
													icon = collapseIcon
												}
												return material.IconButtonStyle{
													Background: theme.ContrastBg,
													Color:      theme.ContrastFg,
													Icon:       icon,
													Size:       unit.Dp(24.0),
													Button:     btn,
													Inset:      layout.UniformInset(unit.Dp(9)),
												}.Layout(gtx)
											},
										)
									}),
								)
							})
						}),
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
