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

//go:generate javac --release 11  -classpath $ANDROID_HOME/platforms/android-36/android.jar -d /tmp/tool_android/classes tool_android.java
//go:generate jar cf tool_android.jar -C /tmp/tool_android/classes .

type PlatformTool struct {
	window         *app.Window
	view           uintptr
	libClass       jni.Class
	askPermission  jni.MethodID
	getExternalDir jni.MethodID
	browseFile     jni.MethodID
}

func (r *PlatformTool) init(env jni.Env) error {
	if r.libClass != 0 {
		return nil // Already initialized
	}

	class, err := jni.LoadClass(env, jni.ClassLoaderFor(env, jni.Object(app.AppContext())), "com/coyace/rtc/tool/tool_android")
	if err != nil {
		return err
	}

	r.libClass = jni.Class(jni.NewGlobalRef(env, jni.Object(class)))
	r.askPermission = jni.GetStaticMethodID(env, r.libClass, "askPermission", "(Landroid/view/View;)V")
	r.getExternalDir = jni.GetStaticMethodID(env, r.libClass, "getExternalDir", "(Landroid/content/Context;)Ljava/lang/String;")
	r.browseFile = jni.GetStaticMethodID(env, r.libClass, "browseFile", "(Landroid/content/Context;Ljava/lang/String;)V")

	return nil
}

func (r *PlatformTool) ListenEvents(evt event.Event) {
	if evt, ok := evt.(app.AndroidViewEvent); ok {
		r.view = evt.View
	}
}

func (r *PlatformTool) AskMicrophonePermission() {
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

func (r *PlatformTool) BrowseFile(path string) {
	err := jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		if err := r.init(env); err != nil {
			return err
		}

		return jni.CallStaticVoidMethod(env, r.libClass, r.browseFile, jni.Value(app.AppContext()), jni.Value(jni.JavaString(env, path)))
	})
	if err != nil {
		log.Println(err)
	}
}

func (r *PlatformTool) GetExternalDir() string {
	if externalDir != "" {
		return externalDir
	}
	err := jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		if err := r.init(env); err != nil {
			return err
		}
		obj, err := jni.CallStaticObjectMethod(env, r.libClass, r.getExternalDir, jni.Value(app.AppContext()))
		if err != nil {
			return err
		}
		if obj != 0 { // jni.Object 是 uintptr
			externalDir = jni.GoString(env, jni.String(obj))
		}
		return nil
	})
	if err != nil {
		log.Println(err)
	}
	return externalDir
}

var (
	externalDir string
)
