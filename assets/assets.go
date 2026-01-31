package assets

import (
	"bytes"
	_ "embed"
	"image"
	_ "image/png"
)

//go:embed icon.png

// AppIcon is app's icon in png format
var AppIcon []byte

// AppIconImage is decoded Image representing AppIcon
var AppIconImage, _, _ = image.Decode(bytes.NewReader(AppIcon))

var Version = "0.0.2"
