package view

import (
	"log"
	"path/filepath"
	"rtc/assets/fonts"
	"rtc/core"
	"runtime"
	"strings"
	"time"

	"gioui.org/layout"
)

func ChooseAndSendPhoto(gtx layout.Context) {
	iconStackAnimation.Disappear(gtx.Now)
	go func() {
		img, gifImg, absolutePath, err := ChooseImageAndDecode()
		if err != nil {
			log.Printf("choose image failed, %v", err)
			return
		}
		filename := filepath.Base(absolutePath)
		// android get displayName, need copy to user space
		if runtime.GOOS == "android" {
			if filepath.Ext(filename) == ".webp" {
				filename = strings.TrimSuffix(filepath.Base(filename), ".webp") + ".png"
			}
			absolutePath = core.GetDataPath(filename)
			go func() {
				if gifImg != nil {
					core.SaveGif(gifImg, filename, false)
				}
				if img != nil {
					core.SaveImg(img, filename, false)
				}
			}()
		}
		message := &Message{State: Stateless, Theme: fonts.DefaultTheme,
			UUID: core.DefaultClient.FullID(), Type: Image, Filename: absolutePath,
			Sender: core.DefaultClient.FullID(), CreatedAt: time.Now()}
		isGif := filepath.Ext(filename) == ".gif"
		if isGif {
			GifCache[absolutePath] = &Gif{GIF: gifImg}
			message.Type = GIF
		}
		MessageBox <- message
		if isGif {
			err = core.DefaultClient.SendGif(gifImg, filename)
		} else {
			imageCache[absolutePath] = &img
			err = core.DefaultClient.SendImage(img, filename)
		}
		if err != nil {
			log.Printf("send image failed, %v", err)
			message.State = Failed
		} else {
			message.State = Sent
		}
	}()
}
