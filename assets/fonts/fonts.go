package fonts

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

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

// 由于gio不支持svg字体，所以嵌入png字体 todo gio支持svg字体
var emoji, _ = opentype.ParseCollection(notoColorEmoji)
var bold, _ = opentype.ParseCollection(robotoBold)
var boldItalic, _ = opentype.ParseCollection(robotoBoldItalic)
var regular, _ = opentype.ParseCollection(robotoRegular)
var regularItalic, _ = opentype.ParseCollection(robotoRegularItalic)

var builtinFonts = [][]font.FontFace{
	gofont.Collection(),
	boldItalic,
	regularItalic,
	bold,
	regular,
	emoji,
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

var SystemFonts = []string{
	"/system/fonts/NotoSansCJK-Regular.ttc",   // Android
	"/System/Library/Fonts/Core/PingFang.ttc", // IOS
	"C:\\Windows\\Fonts\\msyh.ttc",            // Microsoft YaHei
	"C:\\Windows\\Fonts\\msyhbd.ttc",          // Microsoft YaHei Bold
}

func merge() []text.FontFace {
	ret := make([]text.FontFace, 0)
	for _, f := range builtinFonts {
		ret = append(ret, f...)
	}
	for _, fontPath := range MacOsPingFang() {
		ret = tryAdd(ret, fontPath)
	}
	for _, fontPath := range SystemFonts {
		ret = tryAdd(ret, fontPath)
	}
	return ret
}

func tryAdd(ret []text.FontFace, fontPath string) []text.FontFace {
	TTF := tryLoad(fontPath)
	if TTF != nil {
		var parsedFonts, _ = opentype.ParseCollection(TTF)
		ret = append(ret, parsedFonts...)
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

func MacOsPingFang() []string {
	targets := []string{
		"PingFang.ttc",
		"PingFangSC.ttc",
	}

	baseDir := "/System/Library/AssetsV2"
	searchDirs, err := findDirs(baseDir, "com_apple_MobileAsset_Font")
	if err != nil {
		log.Printf("search failed: %v", err)
		return []string{}
	}

	return find(targets, searchDirs)
}

func find(targets []string, searchDirs []string) []string {
	var foundPaths []string
	for _, target := range targets {
		for _, dir := range searchDirs {
			paths, err := findFontFiles(dir, target)
			if err != nil {
				log.Printf("find font failed: %v", err)
				continue
			}
			foundPaths = append(foundPaths, paths...)
		}

		if len(foundPaths) == 0 {
			// 尝试在系统字体目录查找
			paths, _ := findFontFiles("/System/Library/Fonts", target)
			if len(paths) > 0 {
				for _, path := range paths {
					foundPaths = append(foundPaths, path)
				}
			}
		}
	}

	return foundPaths
}

func findDirs(baseDir string, prefix string) ([]string, error) {
	var dirs []string

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			dirs = append(dirs, filepath.Join(baseDir, entry.Name()))
		}
	}

	return dirs, nil
}

func findFontFiles(dir, filename string) ([]string, error) {
	var foundPaths []string

	// 首先检查目录是否存在
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%v not exist", dir)
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// 处理权限错误
			if os.IsPermission(err) {
				// 尝试以只读方式继续
				return nil
			}
			return err
		}

		// 检查文件名是否匹配
		if !info.IsDir() && matchFilename(info.Name(), filename) {
			foundPaths = append(foundPaths, path)
		}

		return nil
	})

	return foundPaths, err
}

func matchFilename(actual, target string) bool {
	// 不区分大小写匹配
	actualLower := strings.ToLower(actual)
	targetLower := strings.ToLower(target)

	// 精确匹配或包含匹配
	if actualLower == targetLower {
		return true
	}
	return false
}

// theme defines the material design style
var DefaultTheme = NewTheme()
