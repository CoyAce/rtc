package native

import (
	"gioui.org/app"
)

var DefaultRecorder *Recorder

func NewRecorder(w *app.Window) (r *Recorder) {
	r = &Recorder{
		window: w,
	}
	return r
}
