package core

import (
	"bytes"
	"image/png"
	"io"
	"log"
	"os"
	"rtc/assets"
	"testing"
)

func TestMap(t *testing.T) {
	var files map[uint32][]Data
	files = make(map[uint32][]Data)
	files[0] = append(files[0], Data{FileId: 0})
	if files[0][0].FileId != 0 {
		t.Fatal("file id should be 0")
	}
}

func TestPNG(t *testing.T) {
	t.Skip("manual test")
	buf := new(bytes.Buffer)
	err := png.Encode(buf, assets.AppIconImage)

	file, err := os.OpenFile("test.png", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer file.Close()

	reader := io.Reader(buf)
	// 使用io.Copy将multiReader的内容写入文件
	if _, err := io.Copy(file, reader); err != nil {
		log.Fatalf("error writing to file: %v", err)
	}
}

func TestSendAndReceive(t *testing.T) {
	//t.Skip("manual test")
	buf := new(bytes.Buffer)
	png.Encode(buf, assets.AppIconImage)
	data := Data{FileId: 0, Payload: bytes.NewReader(buf.Bytes())}
	results := make([]Data, 0)
	for {
		pkt, _ := data.Marshal()
		var d Data
		d.Unmarshal(pkt)
		buffer := d.Payload.(*bytes.Buffer)
		results = append(results, d)
		if buffer.Len() < BlockSize {
			break
		}
	}
	filename := "testX.png"
	removeFile(filename)
	for i, d := range results {
		results = append(results, d)
		if i%15 == 0 {
			d := results[8]
			results[8] = results[10]
			results[9] = d
			write("", filename, results)
			results = make([]Data, 0)
		}
	}
	write("", filename, results)
}
