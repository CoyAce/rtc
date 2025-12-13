package ui

import (
	"rtc/assets/fonts"
	"rtc/core"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/x/component"
)

func Draw(window *app.Window, client *core.Client) error {
	// theme defines the material design style
	theme := fonts.NewTheme()
	// ops are the operations from the UI
	var ops op.Ops

	var messageList = &MessageList{List: layout.List{Axis: layout.Vertical}, Theme: theme}
	// listen for events in the messages channel
	go func() {
		for m := range client.SignedMessages {
			message := Message{Theme: theme, State: Sent, UUID: client.UUID, Sender: m.UUID,
				Text: string(m.Payload), CreatedAt: time.Now()}
			message.AddTo(messageList)
			messageList.ScrollToEnd = true
			window.Invalidate()
		}
	}()

	inputField := component.TextField{Editor: widget.Editor{Submit: true}}
	messageEditor := MessageEditor{InputField: &inputField, Theme: theme}
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
			if messageEditor.Submitted(gtx) {
				msg := strings.TrimSpace(inputField.Text())
				if !client.Connected || client.SendText(msg) != nil {
					message := Message{Theme: theme, Sender: client.UUID, UUID: client.UUID,
						Text: msg, CreatedAt: time.Now(), State: Stateless}
					message.AddTo(messageList)
					messageList.ScrollToEnd = true
				}
				inputField.Clear()
			}

			flex := layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}
			flex.Layout(gtx,
				layout.Flexed(1, messageList.Layout),
				layout.Rigid(messageEditor.Layout),
			)

			// Pass the drawing operations to the GPU.
			e.Frame(gtx.Ops)
		}
	}
}
