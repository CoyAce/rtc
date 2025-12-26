package view

import (
	"fmt"
	"image/gif"
	"os"
	"testing"
)

func TestGifDecode(t *testing.T) {
	t.Skip("manual test")
	file, err := os.Open("/Users/liuhongliang/Downloads/Picture/1.gif")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	gifImg, err := gif.DecodeAll(file)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("img info: %v\n", gifImg)
}
