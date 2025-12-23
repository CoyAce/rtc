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

//go:embed NotoSansSC-Bold.ttf
var notoSansSCBold []byte

//go:embed NotoSansSC-Regular.ttf
var notoSansSCRegular []byte

//go:embed Roboto-Bold.ttf
var robotoBold []byte

//go:embed Roboto-Bold-Italic.ttf
var robotoBoldItalic []byte

//go:embed Roboto-Regular.ttf
var robotoRegular []byte

//go:embed Roboto-Regular-Italic.ttf
var robotoRegularItalic []byte

var notoEmoji, _ = opentype.ParseCollection(notoColorEmoji)
var bold, _ = opentype.ParseCollection(robotoBold)
var boldItalic, _ = opentype.ParseCollection(robotoBoldItalic)
var regular, _ = opentype.ParseCollection(robotoRegular)
var regularItalic, _ = opentype.ParseCollection(robotoRegularItalic)
var scBold, _ = opentype.ParseCollection(notoSansSCBold)
var scRegular, _ = opentype.ParseCollection(notoSansSCRegular)

var arr = [][]font.FontFace{
	gofont.Collection(),
	boldItalic,
	regularItalic,
	bold,
	regular,
	notoEmoji,
	scCollection,
}

var scCollection = []text.FontFace{
	WithStyle(scBold[0], font.Italic),
	WithStyle(scRegular[0], font.Italic),
	scBold[0],
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
