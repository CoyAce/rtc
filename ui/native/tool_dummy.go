//go:build !android

package native

import (
	"gioui.org/app"
	"gioui.org/io/event"
)

type PlatformTool struct {
	window *app.Window
}

func (r *PlatformTool) ListenEvents(evt event.Event) {
}

func (r *PlatformTool) AskMicrophonePermission() {
}
