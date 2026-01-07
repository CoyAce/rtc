//go:build !android

package native

import (
	"gioui.org/app"
	"gioui.org/io/event"
)

type Recorder struct {
	window *app.Window
}

func (r *Recorder) ListenEvents(evt event.Event) {
}

func (r *Recorder) AskPermission() {
}
