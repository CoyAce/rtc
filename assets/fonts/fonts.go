package fonts

import (
	_ "embed"

	"gioui.org/font"
	"gioui.org/font/gofont"
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

var notoEmoji, _ = opentype.ParseCollection(notoColorEmoji)
var black, _ = opentype.ParseCollection(robotoBlack)
var blackItalic, _ = opentype.ParseCollection(robotoBlackItalic)
var bold, _ = opentype.ParseCollection(robotoBold)
var boldItalic, _ = opentype.ParseCollection(robotoBoldItalic)
var light, _ = opentype.ParseCollection(robotoLight)
var lightItalic, _ = opentype.ParseCollection(robotoLightItalic)
var medium, _ = opentype.ParseCollection(robotoMedium)
var mediumItalic, _ = opentype.ParseCollection(robotoMediumItalic)
var regular, _ = opentype.ParseCollection(robotoRegular)
var regularItalic, _ = opentype.ParseCollection(robotoRegularItalic)
var scBlack, _ = opentype.ParseCollection(notoSansSCBlack)
var scBold, _ = opentype.ParseCollection(notoSansSCBold)
var scLight, _ = opentype.ParseCollection(notoSansSCLight)
var scMedium, _ = opentype.ParseCollection(notoSansSCMedium)
var scRegular, _ = opentype.ParseCollection(notoSansSCRegular)

var arr = [][]font.FontFace{
	gofont.Collection(),
	blackItalic,
	boldItalic,
	lightItalic,
	mediumItalic,
	regularItalic,
	black,
	bold,
	light,
	medium,
	regular,
	notoEmoji,
	scCollection,
}

var scCollection = []text.FontFace{
	WithStyle(scBlack[0], font.Italic),
	WithStyle(scBold[0], font.Italic),
	WithStyle(scLight[0], font.Italic),
	WithStyle(scMedium[0], font.Italic),
	WithStyle(scRegular[0], font.Italic),
	scBlack[0],
	scBold[0],
	scLight[0],
	scMedium[0],
	scRegular[0],
}

var collection = merge()

func NewTheme() *material.Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(collection))
	th.Bg.R = 245
	th.Bg.G = 245
	th.Bg.B = 255
	return th
}

func WithStyle(f text.FontFace, style font.Style) text.FontFace {
	f.Font.Style = style
	return f
}

func merge() []text.FontFace {
	ret := make([]text.FontFace, 0)
	for _, f := range arr {
		ret = append(ret, f...)
	}
	return ret
}

// theme defines the material design style
var DefaultTheme = NewTheme()
