package view

import (
	"fmt"
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
	_ = wi.DefaultClient.PublishContent(Content(fd.Path), fd.Name, uint64(fd.Size), fd.ID)
}

func Content(path string) func() (io.ReadSeekCloser, error) {
	return func() (io.ReadSeekCloser, error) {
		r, err := Picker.ReadFile(path)
		if err != nil {
			return nil, err
		}
		f, ok := r.(io.ReadSeekCloser)
		if !ok {
			return nil, fmt.Errorf("current os not support io.ReadSeekCloser")
		}
		return f, nil
	}
}
