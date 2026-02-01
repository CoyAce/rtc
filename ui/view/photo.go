package view

import (
	"log"
	"path/filepath"
	"rtc/assets/fonts"
	"runtime"
	"strings"
	"time"

	"gioui.org/layout"
	"github.com/CoyAce/whily"
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
			absolutePath = GetDataPath(filename)
			go func() {
				if gifImg != nil {
					SaveGif(gifImg, filename, false)
				}
				if img != nil {
					SaveImg(img, filename, false)
				}
			}()
		}
		message := &Message{State: Stateless, Theme: fonts.DefaultTheme,
			UUID: whily.DefaultClient.FullID(), Type: Image, ExternalFilePath: absolutePath,
			Sender: whily.DefaultClient.FullID(), CreatedAt: time.Now()}
		isGif := filepath.Ext(filename) == ".gif"
		if isGif {
			GifCache[absolutePath] = &Gif{GIF: gifImg}
			message.Type = GIF
		}
		MessageBox <- message
		if isGif {
			err = whily.DefaultClient.SendGif(gifImg, filename)
		} else {
			*ImgCache.Load(absolutePath) = img
			err = whily.DefaultClient.SendImage(img, filename)
		}
		if err != nil {
			log.Printf("send image failed, %v", err)
			message.State = Failed
		} else {
			message.State = Sent
		}
	}()
}
