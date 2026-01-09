package fonts

import (
	_ "embed"
	"log"
	"os"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"gioui.org/widget/material"
)

//go:embed NotoColorEmoji.ttf
var notoColorEmoji []byte

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

var arr = [][]font.FontFace{
	gofont.Collection(),
	boldItalic,
	regularItalic,
	bold,
	regular,
	notoEmoji,
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

var fonts = []string{
	"/system/fonts/NotoSansCJK-Regular.ttc",
	"/system/fonts/NotoSansCJK-Bold.ttc",
}

func merge() []text.FontFace {
	ret := make([]text.FontFace, 0)
	for _, f := range arr {
		ret = append(ret, f...)
	}
	for _, fontPath := range fonts {
		ret = tryAdd(ret, fontPath)
	}
	return ret
}

func tryAdd(ret []text.FontFace, fontPath string) []text.FontFace {
	fontBytes := load(fontPath)
	if fontBytes != nil {
		var parsedFonts, _ = opentype.ParseCollection(fontBytes)
		ret = append(ret, parsedFonts...)
		ret = append(ret, WithStyle(parsedFonts[0], font.Italic))
	}
	return ret
}

func load(path string) []byte {
	_, err := os.Stat(path)
	if err != nil {
		log.Printf("load %v failed: %v", path, err)
		return nil
	}
	buf, err := os.ReadFile(path)
	if err != nil {
		log.Printf("load %v failed: %v", path, err)
		return nil
	}
	return buf
}

// theme defines the material design style
var DefaultTheme = NewTheme()
