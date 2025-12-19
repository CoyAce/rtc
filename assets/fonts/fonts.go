package fonts

import (
	_ "embed"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"gioui.org/widget/material"
)

//go:embed NotoColorEmoji.ttf
var notoColorEmoji []byte

//go:embed NotoSansSC-Black.ttf
var notoSansSCBlack []byte

//go:embed NotoSansSC-Bold.ttf
var notoSansSCBold []byte

//go:embed NotoSansSC-Light.ttf
var notoSansSCLight []byte

//go:embed NotoSansSC-Medium.ttf
var notoSansSCMedium []byte

//go:embed NotoSansSC-Regular.ttf
var notoSansSCRegular []byte

//go:embed NotoSansSC-Thin.ttf
var notoSansSCThin []byte

//go:embed Roboto-Black.ttf
var robotoBlack []byte

//go:embed Roboto-Black-Italic.ttf
var robotoBlackItalic []byte

//go:embed Roboto-Bold.ttf
var robotoBold []byte

//go:embed Roboto-Bold-Italic.ttf
var robotoBoldItalic []byte

//go:embed Roboto-Light.ttf
var robotoLight []byte

//go:embed Roboto-Light-Italic.ttf
var robotoLightItalic []byte

//go:embed Roboto-Medium.ttf
var robotoMedium []byte

//go:embed Roboto-Medium-Italic.ttf
var robotoMediumItalic []byte

//go:embed Roboto-Regular.ttf
var robotoRegular []byte

//go:embed Roboto-Regular-Italic.ttf
var robotoRegularItalic []byte

//go:embed Roboto-Thin.ttf
var robotoThin []byte

//go:embed Roboto-Thin-Italic.ttf
var robotoThinItalic []byte

var notoEmoji, _ = opentype.Parse(notoColorEmoji)
var black, _ = opentype.Parse(robotoBlack)
var scBlack, _ = opentype.Parse(notoSansSCBlack)
var blackItalic, _ = opentype.Parse(robotoBlackItalic)
var bold, _ = opentype.Parse(robotoBold)
var scBold, _ = opentype.Parse(notoSansSCBold)
var boldItalic, _ = opentype.Parse(robotoBoldItalic)
var light, _ = opentype.Parse(robotoLight)
var scLight, _ = opentype.Parse(notoSansSCLight)
var lightItalic, _ = opentype.Parse(robotoLightItalic)
var medium, _ = opentype.Parse(robotoMedium)
var scMedium, _ = opentype.Parse(notoSansSCMedium)
var mediumItalic, _ = opentype.Parse(robotoMediumItalic)
var regular, _ = opentype.Parse(robotoRegular)
var scRegular, _ = opentype.Parse(notoSansSCRegular)
var regularItalic, _ = opentype.Parse(robotoRegularItalic)
var thin, _ = opentype.Parse(robotoThin)
var scThin, _ = opentype.Parse(notoSansSCThin)
var thinItalic, _ = opentype.Parse(robotoThinItalic)

var emoji = font.Font{Typeface: "Noto Color Emoji", Weight: font.Normal, Style: font.Regular}
var scBlackFont = font.Font{Typeface: "sans-serif", Weight: font.Black, Style: font.Regular}
var scBlackItalicFont = font.Font{Typeface: "sans-serif", Weight: font.Black, Style: font.Italic}
var scBoldFont = font.Font{Typeface: "sans-serif", Weight: font.Bold, Style: font.Regular}
var scBoldItalicFont = font.Font{Typeface: "sans-serif", Weight: font.Bold, Style: font.Italic}
var scLightFont = font.Font{Typeface: "sans-serif", Weight: font.Light, Style: font.Regular}
var scLightItalicFont = font.Font{Typeface: "sans-serif", Weight: font.Light, Style: font.Italic}
var scMediumFont = font.Font{Typeface: "sans-serif", Weight: font.Medium, Style: font.Regular}
var scMediumItalicFont = font.Font{Typeface: "sans-serif", Weight: font.Medium, Style: font.Italic}
var scRegularFont = font.Font{Typeface: "sans-serif", Weight: font.Normal, Style: font.Regular}
var scRegularItalicFont = font.Font{Typeface: "sans-serif", Weight: font.Normal, Style: font.Italic}
var scThinFont = font.Font{Typeface: "sans-serif", Weight: font.Thin, Style: font.Regular}
var scThinItalicFont = font.Font{Typeface: "sans-serif", Weight: font.Thin, Style: font.Italic}

var blackFont = font.Font{Typeface: "roboto,sans-serif", Weight: font.Black, Style: font.Regular}
var blackItalicFont = font.Font{Typeface: "roboto,sans-serif", Weight: font.Black, Style: font.Italic}
var boldFont = font.Font{Typeface: "roboto,sans-serif", Weight: font.Bold, Style: font.Regular}
var boldItalicFont = font.Font{Typeface: "roboto,sans-serif", Weight: font.Bold, Style: font.Italic}
var lightFont = font.Font{Typeface: "roboto,sans-serif", Weight: font.Light, Style: font.Regular}
var lightItalicFont = font.Font{Typeface: "roboto,sans-serif", Weight: font.Light, Style: font.Italic}
var mediumFont = font.Font{Typeface: "roboto,sans-serif", Weight: font.Medium, Style: font.Regular}
var mediumItalicFont = font.Font{Typeface: "roboto,sans-serif", Weight: font.Medium, Style: font.Italic}
var regularFont = font.Font{Typeface: "roboto,sans-serif", Weight: font.Normal, Style: font.Regular}
var regularItalicFont = font.Font{Typeface: "roboto,sans-serif", Weight: font.Normal, Style: font.Italic}
var thinFont = font.Font{Typeface: "roboto,sans-serif", Weight: font.Thin, Style: font.Regular}
var thinItalicFont = font.Font{Typeface: "roboto,sans-serif", Weight: font.Thin, Style: font.Italic}

var collection = []text.FontFace{
	{Font: blackFont, Face: black},
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
	{Font: emoji, Face: notoEmoji},
	{Font: scBlackItalicFont, Face: scBlack},
	{Font: scBoldItalicFont, Face: scBold},
	{Font: scLightItalicFont, Face: scLight},
	{Font: scMediumItalicFont, Face: scMedium},
	{Font: scRegularItalicFont, Face: scRegular},
	{Font: scThinItalicFont, Face: scThin},
	{Font: scBlackFont, Face: scBlack},
	{Font: scBoldFont, Face: scBold},
	{Font: scLightFont, Face: scLight},
	{Font: scMediumFont, Face: scMedium},
	{Font: scRegularFont, Face: scRegular},
	{Font: scThinFont, Face: scThin},
}

func NewTheme() *material.Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(collection))
	th.Bg.R = 245
	th.Bg.G = 245
	th.Bg.B = 255
	return th
}
