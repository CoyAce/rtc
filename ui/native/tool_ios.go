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
extern void pickPhoto(CFTypeRef pickerRef);
extern void savePhoto(CFTypeRef pickerRef, const char* path);
extern const char* getDocDir(void);
extern void browseFile(CFTypeRef controllerRef,const char* path);
*/
import "C"
import (
	"errors"
	"unsafe"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/x/explorer"
)

type PlatformTool struct {
	window *app.Window
	view   C.CFTypeRef
	picker C.CFTypeRef
}

func (r *PlatformTool) ListenEvents(evt event.Event) {
	switch evt := evt.(type) {
	case app.UIKitViewEvent:
		r.view = C.CFTypeRef(evt.ViewController)
		r.picker = C.createPhotoPicker(r.view)
	}
}

func (r *PlatformTool) AskMicrophonePermission() {
}

func (r *PlatformTool) GetExternalDir() string {
	dir := C.getDocDir()
	defer C.free(unsafe.Pointer(dir))
	return C.GoString(dir)
}

func (r *PlatformTool) BrowseFile(path string) {
	go r.window.Run(func() {
		p := C.CString(path)
		defer C.free(unsafe.Pointer(p))
		C.browseFile(r.view, p)
	})
}

func (r *PlatformTool) ChoosePhoto() (string, error) {
	if r.picker == 0 {
		return "", explorer.ErrNotAvailable
	}
	go r.window.Run(func() {
		C.pickPhoto(r.picker)
	})
	path := <-uc
	if path == "" {
		return "", explorer.ErrUserDecline
	}
	return path, nil
}

func (r *PlatformTool) SavePhoto(path string) error {
	if r.picker == 0 {
		return explorer.ErrNotAvailable
	}

	go r.window.Run(func() {
		p := C.CString(path)
		defer C.free(unsafe.Pointer(p))
		C.savePhoto(r.picker, p)
	})

	err := <-ec
	return err
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

//export exportPhoto
func exportPhoto(errorMsg *C.char) {
	if errorMsg == nil {
		ec <- nil
		return
	}

	// 有错误
	defer C.free(unsafe.Pointer(errorMsg))
	errStr := C.GoString(errorMsg)
	ec <- errors.New(errStr)
}

var uc = make(chan string)
var ec = make(chan error)
