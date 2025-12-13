package fonts

import (
	_ "embed"
	"image/color"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"gioui.org/widget/material"
)

//go:embed black.ttf

var customBlack []byte

//go:embed black-italic.ttf

var customBlackItalic []byte

//go:embed bold.ttf

var customBold []byte

//go:embed bold-italic.ttf

var customBoldItalic []byte

//go:embed light.ttf

var customLight []byte

//go:embed light-italic.ttf

var customLightItalic []byte

//go:embed medium.ttf

var customMedium []byte

//go:embed medium-italic.ttf

var customMediumItalic []byte

//go:embed regular.ttf

var customRegular []byte

//go:embed regular-italic.ttf

var customRegularItalic []byte

//go:embed thin.ttf

var customThin []byte

//go:embed thin-italic.ttf

var customThinItalic []byte

var black, _ = opentype.Parse(customBlack)
var blackItalic, _ = opentype.Parse(customBlackItalic)
var bold, _ = opentype.Parse(customBold)
var boldItalic, _ = opentype.Parse(customBoldItalic)
var light, _ = opentype.Parse(customLight)
var lightItalic, _ = opentype.Parse(customLightItalic)
var medium, _ = opentype.Parse(customMedium)
var mediumItalic, _ = opentype.Parse(customMediumItalic)
var regular, _ = opentype.Parse(customRegular)
var regularItalic, _ = opentype.Parse(customRegularItalic)
var thin, _ = opentype.Parse(customThin)
var thinItalic, _ = opentype.Parse(customThinItalic)

var BlackFont = font.Font{Weight: font.Black, Style: font.Regular}
var blackItalicFont = font.Font{Weight: font.Black, Style: font.Italic}
var boldFont = font.Font{Weight: font.Bold, Style: font.Regular}
var boldItalicFont = font.Font{Weight: font.Bold, Style: font.Italic}
var lightFont = font.Font{Weight: font.Light, Style: font.Regular}
var lightItalicFont = font.Font{Weight: font.Light, Style: font.Italic}
var mediumFont = font.Font{Weight: font.Medium, Style: font.Regular}
var mediumItalicFont = font.Font{Weight: font.Medium, Style: font.Italic}
var regularFont = font.Font{Weight: font.Normal, Style: font.Regular}
var regularItalicFont = font.Font{Weight: font.Normal, Style: font.Italic}
var thinFont = font.Font{Weight: font.Thin, Style: font.Regular}
var thinItalicFont = font.Font{Weight: font.Thin, Style: font.Italic}

var collection = []text.FontFace{
	{Font: BlackFont, Face: black},
	{Font: blackItalicFont, Face: blackItalic},
	{Font: boldFont, Face: bold},
	{Font: boldItalicFont, Face: boldItalic},
	{Font: lightFont, Face: light},
	{Font: lightItalicFont, Face: lightItalic},
	{Font: mediumFont, Face: medium},
	{Font: mediumItalicFont, Face: mediumItalic},
	{Font: regularFont, Face: regular},
	{Font: regularItalicFont, Face: regularItalic},
	{Font: thinFont, Face: thin},
	{Font: thinItalicFont, Face: thinItalic},
}
var AppColor = color.NRGBA{R: 102, G: 117, B: 127, A: 255}

func NewTheme() *material.Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(collection))
	th.Bg.R = 245
	th.Bg.G = 245
	th.Bg.B = 255
	return th
}
