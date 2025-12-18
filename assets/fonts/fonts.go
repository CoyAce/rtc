package fonts

import (
	_ "embed"
	"image/color"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"gioui.org/widget/material"
)

//go:embed NotoEmoji-Black.ttf
var notoEmojiBlack []byte

//go:embed NotoSansSC-Black.ttf
var notoSansSCBlack []byte

//go:embed NotoEmoji-Black-Italic.ttf
var notoEmojiBlackItalic []byte

//go:embed NotoEmoji-Bold.ttf
var notoEmojiBold []byte

//go:embed NotoSansSC-Bold.ttf
var notoSansSCBold []byte

//go:embed NotoEmoji-Bold-Italic.ttf
var notoEmojiBoldItalic []byte

//go:embed NotoEmoji-Light.ttf
var notoEmojiLight []byte

//go:embed NotoSansSC-Light.ttf
var notoSansSCLight []byte

//go:embed NotoEmoji-Light-Italic.ttf
var notoEmojiLightItalic []byte

//go:embed NotoEmoji-Medium.ttf
var notoEmojiMedium []byte

//go:embed NotoSansSC-Medium.ttf
var notoSansSCMedium []byte

//go:embed NotoEmoji-Medium-Italic.ttf
var notoEmojiMediumItalic []byte

//go:embed NotoEmoji-Regular.ttf
var notoEmojiRegular []byte

//go:embed NotoSansSC-Regular.ttf
var notoSansSCRegular []byte

//go:embed NotoEmoji-Regular-Italic.ttf
var notoEmojiRegularItalic []byte

//go:embed NotoEmoji-Thin.ttf
var notoEmojiThin []byte

//go:embed NotoSansSC-Thin.ttf
var notoSansSCThin []byte

//go:embed NotoEmoji-Thin-Italic.ttf
var notoEmojiThinItalic []byte

var black, _ = opentype.Parse(notoEmojiBlack)
var scBlack, _ = opentype.Parse(notoSansSCBlack)
var blackItalic, _ = opentype.Parse(notoEmojiBlackItalic)
var bold, _ = opentype.Parse(notoEmojiBold)
var scBold, _ = opentype.Parse(notoSansSCBold)
var boldItalic, _ = opentype.Parse(notoEmojiBoldItalic)
var light, _ = opentype.Parse(notoEmojiLight)
var scLight, _ = opentype.Parse(notoSansSCLight)
var lightItalic, _ = opentype.Parse(notoEmojiLightItalic)
var medium, _ = opentype.Parse(notoEmojiMedium)
var scMedium, _ = opentype.Parse(notoSansSCMedium)
var mediumItalic, _ = opentype.Parse(notoEmojiMediumItalic)
var regular, _ = opentype.Parse(notoEmojiRegular)
var scRegular, _ = opentype.Parse(notoSansSCRegular)
var regularItalic, _ = opentype.Parse(notoEmojiRegularItalic)
var thin, _ = opentype.Parse(notoEmojiThin)
var scThin, _ = opentype.Parse(notoSansSCThin)
var thinItalic, _ = opentype.Parse(notoEmojiThinItalic)

var blackFont = font.Font{Typeface: "emoji", Weight: font.Black, Style: font.Regular}
var scBlackFont = font.Font{Typeface: "sans-serif", Weight: font.Black, Style: font.Regular}
var blackItalicFont = font.Font{Typeface: "emoji", Weight: font.Black, Style: font.Italic}
var boldFont = font.Font{Typeface: "emoji", Weight: font.Bold, Style: font.Regular}
var scBoldFont = font.Font{Typeface: "sans-serif", Weight: font.Bold, Style: font.Regular}
var boldItalicFont = font.Font{Typeface: "emoji", Weight: font.Bold, Style: font.Italic}
var lightFont = font.Font{Typeface: "emoji", Weight: font.Light, Style: font.Regular}
var scLightFont = font.Font{Typeface: "sans-serif", Weight: font.Light, Style: font.Regular}
var lightItalicFont = font.Font{Typeface: "emoji", Weight: font.Light, Style: font.Italic}
var mediumFont = font.Font{Typeface: "emoji", Weight: font.Medium, Style: font.Regular}
var scMediumFont = font.Font{Typeface: "sans-serif", Weight: font.Medium, Style: font.Regular}
var mediumItalicFont = font.Font{Typeface: "emoji", Weight: font.Medium, Style: font.Italic}
var regularFont = font.Font{Typeface: "emoji", Weight: font.Normal, Style: font.Regular}
var scRegularFont = font.Font{Typeface: "sans-serif", Weight: font.Normal, Style: font.Regular}
var regularItalicFont = font.Font{Typeface: "emoji", Weight: font.Normal, Style: font.Italic}
var thinFont = font.Font{Typeface: "emoji", Weight: font.Thin, Style: font.Regular}
var scThinFont = font.Font{Typeface: "sans-serif", Weight: font.Thin, Style: font.Regular}
var thinItalicFont = font.Font{Typeface: "emoji", Weight: font.Thin, Style: font.Italic}

var collection = []text.FontFace{
	{Font: blackFont, Face: black},
	{Font: scBlackFont, Face: scBlack},
	{Font: blackItalicFont, Face: blackItalic},
	{Font: boldFont, Face: bold},
	{Font: scBoldFont, Face: scBold},
	{Font: boldItalicFont, Face: boldItalic},
	{Font: lightFont, Face: light},
	{Font: scLightFont, Face: scLight},
	{Font: lightItalicFont, Face: lightItalic},
	{Font: mediumFont, Face: medium},
	{Font: scMediumFont, Face: scMedium},
	{Font: mediumItalicFont, Face: mediumItalic},
	{Font: regularFont, Face: regular},
	{Font: scRegularFont, Face: scRegular},
	{Font: regularItalicFont, Face: regularItalic},
	{Font: thinFont, Face: thin},
	{Font: scThinFont, Face: scThin},
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
