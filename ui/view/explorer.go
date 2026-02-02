package view

import (
	"bufio"
	"image"
	"image/draw"
	"image/gif"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"rtc/ui/native"
	"runtime"
	"strings"
	"time"

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
			log.Printf("file name: %v, size: %v, uri: %v", f.Name(), f.Size(), f.URI())
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

func LoadImage(filePath string, reload bool) *image.Image {
	img := ICache.Load(filePath)
	if img != nil && !reload {
		return img
	}
	return ICache.Reload(filePath)
}

func LoadAvatar(filePath string, reload bool) *image.Image {
	img := ACache.Load(filePath)
	if img != nil && !reload {
		return img
	}
	return ACache.Reload(filePath)
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

func LoadGif(filePath string, reload bool) *Gif {
	gifImg := GCache.Load(filePath)
	if gifImg != nil && !reload {
		return gifImg
	}
	return GCache.Reload(filePath)
}

type GifEntry struct {
	path string
	gif  *Gif
	ttl  time.Time
	hit  int
}

type GifCache struct {
	data          []GifEntry
	capacity      int
	ratio         int
	youngCapacity int
	youngHead     int
}

func (g *GifCache) Load(path string) *Gif {
	for i, entry := range g.data {
		if entry.path == path {
			if i < g.youngCapacity {
				g.data[i].hit++
				if g.data[i].hit >= 15 {
					g.promote(i)
				} else {
					g.setTTL(i)
				}
			}
			return entry.gif
		}
	}
	return g.add(path)
}

func (g *GifCache) Reload(path string) *Gif {
	for i, entry := range g.data {
		if entry.path == path {
			gifImg, err := g.load(path)
			if err != nil {
				return g.data[i].gif
			}
			g.data[i].gif = gifImg
			g.setTTL(i)
			return gifImg
		}
	}
	return g.Load(path)
}

func (g *GifCache) promote(i int) {
	idx := rand.Intn(g.capacity-g.youngCapacity) + g.youngCapacity
	temp := g.data[idx]
	temp.hit = 0
	g.data[idx] = g.data[i]
	g.data[i] = temp
}

func (g *GifCache) addYoung(e GifEntry) *Gif {
	g.data[g.youngHead] = e
	g.setTTL(g.youngHead)
	g.youngHead = (g.youngHead + 1) % g.youngCapacity
	return e.gif
}

func (g *GifCache) addDefault(path string) *Gif {
	gifImg := &EmptyGif
	e := GifEntry{path: path, gif: gifImg}
	g.addYoung(e)
	return gifImg
}

func (g *GifCache) add(path string) *Gif {
	gifImg, err := g.load(path)
	if err != nil {
		return g.addDefault(path)
	}
	e := GifEntry{path: path, gif: gifImg}
	return g.addYoung(e)
}

func (g *GifCache) load(path string) (*Gif, error) {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("open %v error: %v", path, err)
		return nil, err
	}
	defer file.Close()

	gifImg, err := gif.DecodeAll(file)
	if err != nil {
		log.Printf("failed to decode gif: %v", err)
		return nil, err
	}
	ret := &Gif{GIF: gifImg}
	return ret, nil
}

func (g *GifCache) setTTL(idx int) {
	ttl := 10 * time.Second
	g.data[idx].ttl = time.Now().Add(ttl)
	time.AfterFunc(ttl, func() {
		if g.data[idx].gif != nil && time.Now().After(g.data[idx].ttl) {
			g.data[idx] = GifEntry{}
		}
	})
}

func (g *GifCache) Reset() {
	g.data = make([]GifEntry, g.capacity)
	g.youngHead = 0
}

func NewGifCache(capacity int, ratio int) *GifCache {
	return &GifCache{
		capacity:      capacity,
		data:          make([]GifEntry, capacity),
		ratio:         ratio,
		youngCapacity: int(float32(capacity) * float32(ratio) / float32(ratio+1)),
	}
}

type ImageEntry struct {
	path string
	img  *image.Image
	ttl  time.Time
	hit  int
}

type ImageCache struct {
	data          []ImageEntry
	capacity      int
	ratio         int
	youngCapacity int
	youngHead     int
}

func (c *ImageCache) Load(path string) *image.Image {
	for i, entry := range c.data {
		if entry.path == path {
			if i < c.youngCapacity {
				c.data[i].hit++
				if c.data[i].hit >= 15 {
					c.promote(i)
				} else {
					c.setTTL(i)
				}
			}
			return entry.img
		}
	}
	return c.add(path)
}

func (c *ImageCache) Reload(path string) *image.Image {
	for i, entry := range c.data {
		if entry.path == path {
			img := c.load(path)
			c.data[i].img = img
			c.setTTL(i)
			return img
		}
	}
	return c.Load(path)
}

func (c *ImageCache) promote(i int) {
	idx := rand.Intn(c.capacity-c.youngCapacity) + c.youngCapacity
	temp := c.data[idx]
	temp.hit = 0
	c.data[idx] = c.data[i]
	c.data[i] = temp
}

func (c *ImageCache) addYoung(e ImageEntry) *image.Image {
	c.data[c.youngHead] = e
	c.setTTL(c.youngHead)
	c.youngHead = (c.youngHead + 1) % c.youngCapacity
	return e.img
}

func (c *ImageCache) setTTL(idx int) {
	ttl := 10 * time.Second
	c.data[idx].ttl = time.Now().Add(ttl)
	time.AfterFunc(ttl, func() {
		if c.data[idx].img != nil && time.Now().After(c.data[idx].ttl) {
			c.data[idx] = ImageEntry{}
		}
	})
}

func (c *ImageCache) add(path string) *image.Image {
	img := c.load(path)
	e := ImageEntry{path: path, img: img}
	return c.addYoung(e)
}

func (c *ImageCache) load(path string) *image.Image {
	ptr := new(image.Image)
	go func() {
		file, err := os.Open(path)
		if err != nil {
			log.Printf("open %v error: %v", path, err)
			ptr = nil
			return
		}
		defer file.Close()

		img, err := decodeImage(file)
		if err != nil {
			log.Printf("failed to decode image: %v", err)
			ptr = nil
			return
		}
		if !IsGPUFriendly(img) {
			img = ConvertToGPUFriendlyImage(img)
		}
		*ptr = img
	}()
	return ptr
}

func (c *ImageCache) Reset() {
	c.data = make([]ImageEntry, c.capacity)
	c.youngHead = 0
}

func NewImageCache(capacity int, ratio int) *ImageCache {
	return &ImageCache{
		capacity:      capacity,
		data:          make([]ImageEntry, capacity),
		ratio:         ratio,
		youngCapacity: int(float32(capacity) * float32(ratio) / float32(ratio+1)),
	}
}

var ACache = NewImageCache(10, 2)
var ICache = NewImageCache(8, 2)
var GCache = NewGifCache(8, 2)
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
