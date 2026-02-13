package native

/*
#cgo CFLAGS: -Werror -xobjective-c -fmodules -fobjc-arc
#import <PhotosUI/PhotosUI.h>
#include <stdint.h>

@interface photo_picker : NSObject <PHPickerViewControllerDelegate>
@property (nonatomic, weak) UIViewController *controller;
- (void)pickPhotos;
@end

extern CFTypeRef createPhotoPicker(CFTypeRef controllerRef);
extern void pickPhotos(CFTypeRef pickerRef);
*/
import "C"
import (
	"unsafe"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/x/explorer"
)

type PlatformTool struct {
	window *app.Window
	picker C.CFTypeRef
}

func (r *PlatformTool) ListenEvents(evt event.Event) {
	switch evt := evt.(type) {
	case app.UIKitViewEvent:
		r.picker = C.createPhotoPicker(C.CFTypeRef(evt.ViewController))
	}
}

func (r *PlatformTool) AskMicrophonePermission() {
}

func (r *PlatformTool) GetExternalDir() string {
	dir, _ := app.DataDir()
	return dir
}

func (r *PlatformTool) BrowseFile(path string) {
}

func (r *PlatformTool) ChoosePhoto() (string, error) {
	if r.picker == 0 {
		return "", explorer.ErrNotAvailable
	}
	go r.window.Run(func() {
		C.pickPhotos(r.picker)
	})
	path := <-uc
	if path == "" {
		return "", explorer.ErrUserDecline
	}
	return path, nil
}

//export importPhoto
func importPhoto(u *C.char) {
	if u == nil {
		uc <- ""
		return
	}
	defer C.free(unsafe.Pointer(u))
	uc <- C.GoString(u)
}

var uc = make(chan string)
