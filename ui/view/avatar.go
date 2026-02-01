package view

import (
	"image"
	"image/gif"
	"log"
	"rtc/assets"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/CoyAce/whily"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

type AvatarType uint8

const (
	Default AvatarType = iota
	IMG
	GIF_IMG
)

type Avatar struct {
	UUID       string
	Size       int
	Editable   bool
	EditButton IconButton
	OnChange   func(img image.Image, gif *gif.GIF)
	point      image.Point
	Image      image.Image
	*Gif
	AvatarType
	widget.Clickable
	*material.Theme
}

func (v *Avatar) Layout(gtx layout.Context) layout.Dimensions {
	if v.Size == 0 {
		v.Size = 48
	}
	v.point = image.Point{X: gtx.Dp(unit.Dp(v.Size)), Y: gtx.Dp(unit.Dp(v.Size))}
	if v.Gif == nil && v.Image == nil {
		v.Reload(Default)
	}
	if v.Editable && v.Clicked(gtx) {
		go func() {
			img, gifImg, _, err := ChooseImageAndDecode()
			if err != nil {
				return
			}

			// update settings and avatar cache
			if img != nil {
				if img.Bounds().Dx() > 512 || img.Bounds().Dy() > 512 {
					img = resizeImage(img, 512, 512)
				}
				v.Image = img
				v.AvatarType = IMG
				avatar := AvatarCache.LoadOrElseNew(whily.DefaultClient.FullID())
				avatar.Image = img
				avatar.AvatarType = IMG
			}
			if gifImg != nil {
				v.Gif = &Gif{GIF: gifImg}
				v.AvatarType = GIF_IMG
				avatar := AvatarCache.LoadOrElseNew(whily.DefaultClient.FullID())
				avatar.Gif = v.Gif
				avatar.AvatarType = GIF_IMG
			}

			// save to file
			if gifImg != nil {
				SaveGif(gifImg, "icon.gif", true)
				whily.RemoveFile(GetPath(v.UUID, "icon.png"))
			} else {
				SaveImg(img, "icon.png", true)
				whily.RemoveFile(GetPath(v.UUID, "icon.gif"))
			}

			// sync to server
			if v.OnChange != nil {
				v.OnChange(img, gifImg)
			}
		}()
	}
	gtx.Constraints.Min, gtx.Constraints.Max = v.point, v.point
	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return v.Clickable.Layout(gtx, func(gtx layout.Context) (d layout.Dimensions) {
				macro := op.Record(gtx.Ops)
				if v.AvatarType == IMG || v.AvatarType == Default {
					imgOps := paint.NewImageOp(v.Image)
					imgWidget := widget.Image{Src: imgOps, Fit: widget.Fill, Position: layout.Center, Scale: 0}
					d = imgWidget.Layout(gtx)
				} else {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					d = v.Gif.Layout(gtx)
				}
				call := macro.Stop()
				xSpace := d.Size.X - gtx.Constraints.Max.X
				ySpace := d.Size.Y - gtx.Constraints.Max.Y
				defer op.Offset(image.Point{X: -xSpace / 2, Y: -ySpace / 2}).Push(gtx.Ops).Pop()
				defer clip.UniformRRect(image.Rectangle{
					Min: image.Point{
						X: xSpace / 2,
						Y: ySpace / 2,
					},
					Max: image.Point{
						X: xSpace/2 + gtx.Constraints.Max.X,
						Y: ySpace/2 + gtx.Constraints.Max.Y,
					},
				}, gtx.Constraints.Max.X/2).Push(gtx.Ops).Pop()
				call.Add(gtx.Ops)
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if !v.Editable || !v.Hovered() {
				return layout.Dimensions{}
			}
			gtx.Constraints.Max.X = int(float64(v.point.X) * 0.40)
			gtx.Constraints.Max.Y = int(float64(v.point.Y) * 0.40)
			gtx.Constraints.Min = gtx.Constraints.Max
			iconClr := v.Theme.ContrastFg
			icon, _ := widget.NewIcon(icons.ImageEdit)
			return icon.Layout(gtx, iconClr)
		}),
	)
}

func (v *Avatar) Reload(avatarType AvatarType) {
	if avatarType == GIF_IMG || avatarType == Default {
		gifPath := GetPath(v.UUID, "icon.gif")
		gifImg, err := LoadGif(gifPath, true)
		if err != nil {
			log.Println("failed to load GIF:", err)
		}
		if gifImg != nil && gifImg != &EmptyGif {
			v.Gif = gifImg
			v.AvatarType = GIF_IMG
			whily.RemoveFile(GetPath(v.UUID, "icon.png"))
			return
		}
	}

	imgPath := GetPath(v.UUID, "icon.png")
	img, err := LoadImage(imgPath, true)
	if err != nil {
		log.Printf("failed to decode icon: %v", err)
	}
	if v.Image == nil {
		v.Image = assets.AppIconImage
	}
	if img != nil {
		v.Image = *img
		v.AvatarType = IMG
		whily.RemoveFile(GetPath(v.UUID, "icon.gif"))
	}
}

func resizeImage(src image.Image, newWidth, newHeight int) image.Image {
	// 创建一个新的RGBA图像，用于存放调整大小后的图像数据
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	// 计算缩放比例因子
	scaleX := float64(newWidth) / float64(src.Bounds().Dx())
	scaleY := float64(newHeight) / float64(src.Bounds().Dy())

	for x := 0; x < newWidth; x++ {
		for y := 0; y < newHeight; y++ {
			// 计算源图像中对应的像素位置（插值）
			srcX := int(float64(x) / scaleX)
			srcY := int(float64(y) / scaleY)
			dst.Set(x, y, src.At(srcX, srcY)) // 直接赋值，不考虑插值，结果可能不够平滑
		}
	}
	return dst
}

type avatarCache struct {
	cache map[string]*Avatar
}

func newAvatarCache() *avatarCache {
	c := &avatarCache{}
	c.cache = make(map[string]*Avatar)
	return c
}

func (c *avatarCache) Add(uuid string, avatar *Avatar) {
	c.cache[uuid] = avatar
}

func (c *avatarCache) LoadOrElseNew(uuid string) *Avatar {
	avatar := c.cache[uuid]
	if avatar == nil {
		avatar = &Avatar{UUID: uuid}
		c.cache[uuid] = avatar
	}
	return avatar
}

var AvatarCache = newAvatarCache()
