//go:build !android

// SPDX-License-Identifier: Unlicense OR MIT

package native

import (
	"io"

	"gioui.org/app"
	"gioui.org/io/event"
)

type explorer struct {
	window *app.Window
	view   uintptr
	result chan result
}

func newExplorer(w *app.Window) *explorer {
	return &explorer{window: w, result: make(chan result)}
}

func (e *Explorer) listenEvents(evt event.Event) {
}

func (e *Explorer) exportFile(name string) (io.WriteCloser, error) {
	return nil, nil
}

func (e *Explorer) importFile(extensions ...string) (io.ReadCloser, error) {
	return nil, nil
}

func (e *Explorer) importFiles(_ ...string) ([]io.ReadCloser, error) {
	return nil, ErrNotAvailable
}

type File struct {
}

func (f *File) Read(b []byte) (n int, err error) {
	return 0, ErrNotAvailable
}
func (f *File) Write(b []byte) (n int, err error) {
	return 0, ErrNotAvailable
}
func (f *File) Close() error {
	return ErrNotAvailable
}
func (f *File) Name() string { return "" }
func (f *File) Size() int64  { return 0 }
