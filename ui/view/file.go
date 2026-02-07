package view

import (
	"io"
	"log"
	"rtc/assets/fonts"
	"time"
	"unsafe"

	"github.com/CoyAce/wi"
)

func ChooseAndSendFile(appendFile func(description *FileDescription)) func() {
	return func() {
		go func() {
			fd, err := ChooseFile()
			if err != nil {
				log.Printf("Choose file failed: %v", err)
				return
			}
			id := wi.Hash(unsafe.Pointer(&fd))
			fc := FileControl{
				Filename: fd.Name,
				FileId:   id,
				Path:     fd.Path,
				Size:     uint64(fd.Size),
				Mime:     NewMine(fd.Name),
			}
			message := &Message{
				State: Stateless,
				MessageStyle: MessageStyle{
					Theme: fonts.DefaultTheme,
				},
				FileControl: fc,
				Contacts:    FromMyself(),
				MessageType: File,
				CreatedAt:   time.Now(),
			}
			MessageBox <- message
			fd.ID = id
			appendFile(&fd)
			err = wi.DefaultClient.PublishFile(fd.Name, uint64(fd.Size), id)
			if err != nil {
				log.Printf("Publish file failed, %v", err)
				message.State = Failed
			} else {
				message.State = Sent
			}
		}()
	}
}

func PublishContent(fd *FileDescription) {
	r, err := Picker.ReadFile(fd.Path)
	f, ok := r.(io.ReadSeekCloser)
	if err != nil {
		log.Printf("Load file failed %v", err)
		return
	}
	if !ok {
		log.Printf("Current os not support")
		return
	}
	_ = wi.DefaultClient.PublishContent(fd.Name, uint64(fd.Size), fd.ID, f)
}
