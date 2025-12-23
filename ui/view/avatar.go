package view

import (
	"image"
	"log"
	"rtc/assets"
	"rtc/core"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

type Avatar struct {
	UUID          string
	Size          int
	Editable      bool
	EditButton    IconButton
	OnChange      func(img image.Image)
	point         image.Point
	Image         image.Image
	selectedImage chan image.Image
	widget.Clickable
	*material.Theme
}

func (v *Avatar) Layout(gtx layout.Context) layout.Dimensions {
	if v.Size == 0 {
		v.Size = 48
	}
	if v.selectedImage == nil {
		v.selectedImage = make(chan image.Image)
	}
	v.point = image.Point{X: gtx.Dp(unit.Dp(v.Size)), Y: gtx.Dp(unit.Dp(v.Size))}
	if v.Image == nil {
		v.Reload()
	}
	if v.Editable && v.Clicked(gtx) {
		go func() {
			img, _, err := ChooseImageAndDecode()
			if err != nil {
				return
			}
			if img.Bounds().Dx() > 512 || img.Bounds().Dy() > 512 {
				img = resizeImage(img, 512, 512)
			}

			v.selectedImage <- img

			// save to file
			core.Save(img, "icon.png")

			// sync to server
			if v.OnChange != nil {
				v.OnChange(img)
			}
		}()
	}
	if v.Editable {
		select {
		case img, ok := <-v.selectedImage:
			if ok {
				v.Image = img
				avatar := AvatarCache[core.DefaultClient.FullID()]
				if avatar != nil {
					avatar.Image = img
				} else {
					avatar = &Avatar{UUID: core.DefaultClient.FullID(), Image: img}
					AvatarCache[core.DefaultClient.FullID()] = avatar
				}
			}
		default:
		}
	}
	gtx.Constraints.Min, gtx.Constraints.Max = v.point, v.point
	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return v.Clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				imgOps := paint.NewImageOp(v.Image)
				imgWidget := widget.Image{Src: imgOps, Fit: widget.Fill, Position: layout.Center, Scale: 0}
				defer clip.UniformRRect(image.Rectangle{
					Max: image.Point{
						X: gtx.Constraints.Max.X,
						Y: gtx.Constraints.Max.Y,
					},
				}, gtx.Constraints.Max.X/2).Push(gtx.Ops).Pop()
				return imgWidget.Layout(gtx)
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

func (v *Avatar) Reload() {
	filePath := core.GetFilePath(v.UUID, "icon.png")
	img, err := LoadImage(filePath)
	if err != nil {
		log.Printf("failed to decode icon: %v", err)
		if v.Image == nil {
			v.Image = assets.AppIconImage
		}
	}
	if img != nil {
		v.Image = *img
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

var AvatarCache = make(map[string]*Avatar)
