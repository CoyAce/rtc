package view

import (
	"bufio"
	"image"
	"image/draw"
	"image/gif"
	"io"
	"log"
	"os"
	"path/filepath"
	"rtc/assets"
	"rtc/ui/native"
	"runtime"
	"strings"

	"github.com/CoyAce/whily"

	"gioui.org/app"
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
	if IsGPUFriendly(img) {
		imageCache[filePath] = &img
	} else {
		var dst image.Image
		dst = ConvertToGPUFriendlyImage(img)
		imageCache[filePath] = &dst
	}
	return &img, nil
}

func IsGPUFriendly(img image.Image) bool {
	switch img.(type) {
	case *image.Uniform:
		return true
	case *image.RGBA:
		return true
	}
	return false
}

func ConvertToGPUFriendlyImage(src image.Image) *image.RGBA {
	sz := src.Bounds().Size()
	// Copy the image into a GPU friendly format.
	dst := image.NewRGBA(image.Rectangle{
		Max: sz,
	})
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return dst
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

	GIF, err := gif.DecodeAll(file)
	if err != nil {
		GifCache[filePath] = &EmptyGif
		log.Printf("failed to decode gif: %v", err)
		return nil, err
	}

	ret := &Gif{GIF: GIF}
	GifCache[filePath] = ret
	return ret, nil
}

var imageCache = map[string]*image.Image{}
var GifCache = map[string]*Gif{}
var EmptyGif Gif

func GetDataDir() string {
	dir, _ := app.DataDir()
	if runtime.GOOS == "android" {
		return dir
	}
	return dir + "/coyace.rtc/"
}

func GetDir(uuid string) string {
	if uuid == "" {
		return GetDataDir() + "/default"
	}
	return GetDataDir() + "/" + strings.Replace(uuid, "#", "_", -1)
}

func GetPath(uuid string, filename string) string {
	return GetDir(uuid) + "/" + filename
}

func GetDataPath(filename string) string {
	return GetPath(whily.DefaultClient.FullID(), filename)
}

func GetFilePath(filename string) string {
	return GetDataDir() + filename
}

func SaveImg(img image.Image, filename string, rewrite bool) {
	if filepath.Ext(filename) == ".webp" {
		filename = strings.TrimSuffix(filepath.Base(filename), ".webp") + ".png"
	}
	filePath := GetDataPath(filename)
	_, err := os.Stat(filePath)
	if err == nil && !rewrite {
		return
	}
	whily.Mkdir(filepath.Dir(filePath))
	file, err := os.Create(filePath)
	defer file.Close()
	if err != nil {
		log.Printf("create file failed, %v", err)
	}
	err = whily.EncodeImg(file, filePath, img)
	if err != nil {
		log.Printf("encode file failed, %v", err)
	} else {
		log.Printf("%s saved to %s", filename, filePath)
	}
}

func SaveGif(gifImg *gif.GIF, filename string, rewrite bool) {
	filePath := GetDataPath(filename)
	_, err := os.Stat(filePath)
	if err == nil && !rewrite {
		return
	}
	whily.Mkdir(filepath.Dir(filePath))
	file, err := os.Create(filePath)
	defer file.Close()
	if err != nil {
		log.Printf("create file failed, %v", err)
	}
	whily.EncodeGif(file, filename, gifImg)
}
