// SPDX-License-Identifier: Unlicense OR MIT

package native

/*
#cgo LDFLAGS: -landroid

#include <jni.h>
#include <stdlib.h>
*/
import "C"
import (
	"log"

	"gioui.org/app"
	"gioui.org/io/event"
	"git.wow.st/gmp/jni"
)

//go:generate javac --release 11  -classpath $ANDROID_HOME/platforms/android-36/android.jar -d /tmp/recorder_android/classes recorder_android.java
//go:generate jar cf recorder_android.jar -C /tmp/recorder_android/classes .

type Recorder struct {
	window        *app.Window
	view          uintptr
	libClass      jni.Class
	askPermission jni.MethodID
}

func (r *Recorder) init(env jni.Env) error {
	if r.libClass != 0 {
		return nil // Already initialized
	}

	class, err := jni.LoadClass(env, jni.ClassLoaderFor(env, jni.Object(app.AppContext())), "com/coyace/rtc/recorder/recorder_android")
	if err != nil {
		return err
	}

	r.libClass = jni.Class(jni.NewGlobalRef(env, jni.Object(class)))
	r.askPermission = jni.GetStaticMethodID(env, r.libClass, "askPermission", "(Landroid/view/View;)V")

	return nil
}

func (r *Recorder) ListenEvents(evt event.Event) {
	if evt, ok := evt.(app.AndroidViewEvent); ok {
		r.view = evt.View
	}
}

func (r *Recorder) AskPermission() {
	r.window.Run(func() {
		err := jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
			if err := r.init(env); err != nil {
				return err
			}

			return jni.CallStaticVoidMethod(env, r.libClass, r.askPermission, jni.Value(r.view))
		})
		if err != nil {
			log.Println(err)
		}
	})
}
