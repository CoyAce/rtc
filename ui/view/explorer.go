package view

import (
	"bufio"
	"image"
	"image/gif"
	"io"
	"log"
	"os"
	"path/filepath"
	"rtc/assets"
	"rtc/ui/native"
	"runtime"

	"gioui.org/io/event"
	"golang.org/x/image/webp"
)

var DefaultPicker Picker

type Picker interface {
	ListenEvents(evt event.Event)
	ChooseFile(extensions ...string) (io.ReadCloser, error)
	ChooseFiles(extensions ...string) ([]io.ReadCloser, error)
	CreateFile(name string) (io.WriteCloser, error)
}

func ChooseImageAndDecode() (image.Image, *gif.GIF, string, error) {
	file, err := DefaultPicker.ChooseFile(".jpg", ".jpeg", ".png", ".webp", ".gif")
	if err != nil {
		return nil, nil, "", err
	}
	var filename string
	if f, ok := file.(*os.File); ok {
		filename = f.Name()
	}
	if runtime.GOOS == "android" {
		if f, ok := file.(*native.File); ok {
			log.Printf("file name: %v, size: %v", f.Name(), f.Size())
			filename = f.Name()
		}
	}
	defer file.Close()

	if filepath.Ext(filename) == ".gif" {
		img, err := decodeGif(file)
		if err != nil {
			return nil, nil, filename, err
		}
		return nil, img, filename, nil
	}

	img, err := decodeImage(file)
	if err != nil {
		return nil, nil, filename, err
	}
	return img, nil, filename, nil
}

func decodeGif(file io.ReadCloser) (*gif.GIF, error) {
	gif, err := gif.DecodeAll(file)
	if err != nil {
		return nil, err
	}
	return gif, nil
}

func decodeImage(file io.ReadCloser) (image.Image, error) {
	var img, _, err = image.Decode(bufio.NewReader(file))
	if img == nil {
		// try with webp
		img, err = webp.Decode(bufio.NewReader(file))
	}
	return img, err
}

func LoadImage(filePath string, reload bool) (*image.Image, error) {
	if imageCache[filePath] != nil && !reload {
		return imageCache[filePath], nil
	}
	file, err := os.Open(filePath)
	if err != nil {
		imageCache[filePath] = &assets.AppIconImage
		log.Printf("open %v error: %v", filePath, err)
		return nil, err
	}
	defer file.Close()

	img, err := decodeImage(file)
	if err != nil {
		imageCache[filePath] = &assets.AppIconImage
		log.Printf("failed to decode image: %v", err)
		return nil, err
	}
	imageCache[filePath] = &img
	return &img, nil
}

func LoadGif(filePath string, reload bool) (*Gif, error) {
	if GifCache[filePath] != nil && !reload {
		return GifCache[filePath], nil
	}
	file, err := os.Open(filePath)
	if err != nil {
		GifCache[filePath] = &EmptyGif
		log.Printf("open %v error: %v", filePath, err)
		return nil, err
	}
	defer file.Close()

	gif, err := gif.DecodeAll(file)
	if err != nil {
		GifCache[filePath] = &EmptyGif
		log.Printf("failed to decode gif: %v", err)
		return nil, err
	}

	ret := &Gif{GIF: gif}
	GifCache[filePath] = ret
	return ret, nil
}

var imageCache = map[string]*image.Image{}
var GifCache = map[string]*Gif{}
var EmptyGif Gif
