package core

import (
	"bytes"
	"image/png"
	"io"
	"log"
	"os"
	"reflect"
	"rtc/assets"
	"testing"
)

func TestRange(t *testing.T) {
	rt := RangeTracker{}
	rt.Add(Range{1, 20})
	if !rt.isCompleted() {
		t.Error("Expected RangeTracker to be completed")
	}
}

func TestRangeRemove(t *testing.T) {
	rt := RangeTracker{}
	rt.Add(Range{1, 20})
	rt.Add(Range{21, 21})
	// miss 22
	rt.Add(Range{23, 24})
	// miss 22, [25,49]
	rt.Add(Range{50, 50})
	expected := []Range{{22, 22}, {25, 49}}
	if !reflect.DeepEqual(rt.GetRanges(), expected) {
		t.Errorf("Expected RangeTracker %v, got %v", expected, rt.GetRanges())
	}
	rt.Add(Range{20, 22})
	expected = []Range{{25, 49}}
	if !reflect.DeepEqual(rt.GetRanges(), expected) {
		t.Errorf("Expected RangeTracker %v, got %v", expected, rt.GetRanges())
	}
	rt.Add(Range{25, 25})
	expected = []Range{{26, 49}}
	if !reflect.DeepEqual(rt.GetRanges(), expected) {
		t.Errorf("Expected RangeTracker %v, got %v", expected, rt.GetRanges())
	}
	rt.Add(Range{45, 50})
	rt.Add(Range{20, 20})
	expected = []Range{{26, 44}}
	if !reflect.DeepEqual(rt.GetRanges(), expected) {
		t.Errorf("Expected RangeTracker %v, got %v", expected, rt.GetRanges())
	}
	rt.Add(Range{25, 30})
	expected = []Range{{31, 44}}
	if !reflect.DeepEqual(rt.GetRanges(), expected) {
		t.Errorf("Expected RangeTracker %v, got %v", expected, rt.GetRanges())
	}
	// miss [31,35],[40,44]
	rt.Add(Range{36, 39})
	expected = []Range{{31, 35}, {40, 44}}
	if !reflect.DeepEqual(rt.GetRanges(), expected) {
		t.Errorf("Expected RangeTracker %v, got %v", expected, rt.GetRanges())
	}
	rt.Add(Range{38, 38})
	rt.Add(Range{31, 35})
	rt.Add(Range{40, 44})
	if !rt.isCompleted() {
		t.Error("Expected RangeTracker to be completed")
	}
}

func TestRangeAdd(t *testing.T) {
	rt := RangeTracker{}
	rt.Add(Range{1, 20})
	rt.Add(Range{1, 20})
	rt.Add(Range{30, 40})
	rt.Add(Range{40, 50})
	rt.Add(Range{40, 45})
	expected := []Range{{21, 29}}
	if !reflect.DeepEqual(rt.GetRanges(), expected) {
		t.Errorf("Expected RangeTracker %v, got %v", expected, rt.GetRanges())
	}
	rt.Add(rt.ranges[0])
	if !rt.isCompleted() {
		t.Error("Expected RangeTracker to be completed")
	}
}

func TestNckMarshalAndUnmarshal(t *testing.T) {
	ranges := []Range{{1, 1}, {3, 5}, {8, 8}}
	nck := Nck{FileId: 1, ranges: ranges}
	pkt, err := nck.Marshal()
	if err != nil {
		t.Error(err)
	}
	var ret Nck
	err = ret.Unmarshal(pkt)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(nck, ret) {
		t.Errorf("Expected %v, got %v", nck, ret)
	}
}

func TestCircularBuffer(t *testing.T) {
	cb := NewCircularBuffer(5)
	for i := 0; i < 5; i++ {
		cb.Write(Packet{Block: uint32(i + 1)})
	}
	ret := cb.Read([]Range{{1, 2}, {4, 4}})
	expected := []Packet{{Block: uint32(1)}, {Block: uint32(2)}, {Block: uint32(4)}}
	if !reflect.DeepEqual(ret, expected) {
		t.Errorf("Expected %v, got %v", expected, ret)
	}
}

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

func TestConsecutive(t *testing.T) {
	d := []Data{{Block: 1}, {Block: 2}, {Block: 3}, {Block: 5}}
	i := findConsecutive(d)
	if i != 3 {
		t.Fatal("consecutive function error")
	}
}

func TestSendAndReceive(t *testing.T) {
	t.Skip("manual test")
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
	RemoveFile(filename)
	write(filename, results[:1])
}
