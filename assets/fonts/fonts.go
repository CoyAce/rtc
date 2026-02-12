package fonts

import (
	_ "embed"
	"log"
	"os"
	"rtc/assets/fonts/notoemoji"
	"rtc/assets/fonts/notosanssc"
	"rtc/assets/fonts/roboto"
	"runtime"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"gioui.org/widget/material"
)

var collection = mergeAndClean()

func builtinFonts() [][]font.FontFace {
	var emoji, _ = opentype.ParseCollection(notoemoji.TTF)
	var bold, _ = opentype.ParseCollection(roboto.BOLD)
	var boldItalic, _ = opentype.ParseCollection(roboto.BOLD_ITALIC)
	var regular, _ = opentype.ParseCollection(roboto.REGULAR)
	var regularItalic, _ = opentype.ParseCollection(roboto.REGULAR_ITALIC)
	return [][]font.FontFace{
		gofont.Collection(),
		boldItalic,
		regularItalic,
		bold,
		regular,
		emoji,
	}
}

func NewTheme() *material.Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(collection))
	th.Bg.R = 245
	th.Bg.G = 245
	th.Bg.B = 255
	return th
}

var SystemFonts = []string{
	"/system/fonts/VivoFont.ttf",
	"/system/fonts/MiSans.ttf",
	"/system/fonts/HwSans.ttf",
	"/system/fonts/HwChinese.ttf",
	"/system/fonts/MeizuSans.ttf",
	"/system/fonts/MeizuChinese.ttf",
	"/system/fonts/OPPOSans.ttf",
	"/system/fonts/SamsungOne.ttf",
	"/system/fonts/SamsungChinese.ttf",
	"/system/fonts/OnePlusSans.ttf",
	"/system/fonts/realmeSans.ttf",
	"/system/fonts/HonorSans.ttf",
	"/system/fonts/GoogleSans.ttf",
	"C:\\Windows\\Fonts\\msyh.ttc",   // Microsoft YaHei
	"C:\\Windows\\Fonts\\msyhbd.ttc", // Microsoft YaHei Bold
}

func WithStyle(f text.FontFace, style font.Style) text.FontFace {
	f.Font.Style = style
	return f
}

func mergeAndClean() []text.FontFace {
	defer runtime.GC()
	defer clean()
	return merge()
}

func merge() []text.FontFace {
	ret := make([]text.FontFace, 0)
	weights := []font.Weight{font.Normal, font.Bold}
	for _, f := range builtinFonts() {
		ret = append(ret, f...)
	}
	for _, fontPath := range SystemFonts {
		ret = tryAdd(ret, fontPath, weights)
	}
	for _, f := range NotoSansSC() {
		ret = append(ret, f...)
	}
	return ret
}

func NotoSansSC() [][]font.FontFace {
	var scRegular, _ = opentype.ParseCollection(notosanssc.REGULAR)
	var scBold, _ = opentype.ParseCollection(notosanssc.BOLD)
	return [][]font.FontFace{
		scRegular,
		scBold,
	}
}

func clean() {
	roboto.BOLD = nil
	roboto.BOLD_ITALIC = nil
	roboto.REGULAR = nil
	roboto.REGULAR_ITALIC = nil
	notoemoji.TTF = nil
	notosanssc.REGULAR = nil
	notosanssc.BOLD = nil
}

func tryAdd(ret []text.FontFace, fontPath string, weights []font.Weight) []text.FontFace {
	TTF := tryLoad(fontPath)
	if TTF != nil {
		var parsedFonts, _ = opentype.ParseCollection(TTF)
		filteredFonts := filter(parsedFonts, weights)
		ret = append(ret, filteredFonts...)
		for _, f := range filteredFonts {
			ret = append(ret, WithStyle(f, font.Italic))
		}
	}
	return ret
}

func filter(fonts []text.FontFace, weights []font.Weight) []text.FontFace {
	ret := make([]text.FontFace, 0, len(weights))
	for _, w := range weights {
		for _, f := range fonts {
			if f.Font.Weight == w {
				ret = append(ret, f)
			}
		}
	}
	return ret
}

func tryLoad(path string) []byte {
	_, err := os.Stat(path)
	if err != nil {
		return nil
	}
	buf, err := os.ReadFile(path)
	if err != nil {
		log.Printf("load %v failed: %v", path, err)
		return nil
	}
	return buf
}

// DefaultTheme defines the material design style
var DefaultTheme = NewTheme()
