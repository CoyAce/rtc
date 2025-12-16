package core

import (
	"bytes"
	"log"
	"os"
	"strings"
)

var dataDir = "data/"

func GetDir(uuid string) string {
	return dataDir + strings.Replace(uuid, "#", "_", -1)
}

func GetFileName(uuid string, filename string) string {
	if uuid == "" {
		return dataDir + filename
	}
	return GetDir(uuid) + "/" + filename
}

func RemoveFile(filePath string) {
	// 使用os.Stat检查文件是否存在
	_, err := os.Stat(filePath)
	if err == nil {
		// 文件存在，尝试删除
		err := os.Remove(filePath)
		if err != nil {
			log.Printf("Error removing file: %s\n", err)
		}
	}
}

func removeDuplicates(data []Data) []Data {
	seen := make(map[uint32]bool)
	result := []Data{}
	for _, d := range data {
		if !seen[d.Block] {
			seen[d.Block] = true
			result = append(result, d)
		}
	}
	return result
}

func Mkdir(dir string) {
	if len(dir) == 0 {
		return
	}
	// 使用 MkdirAll 确保目录存在
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		log.Fatalf("Error creating directory: %v", err)
	}
}

func writeString(b *bytes.Buffer, str string) error {
	_, err := b.WriteString(str) // write str
	if err != nil {
		return err
	}

	err = b.WriteByte(0) // write 0 byte
	if err != nil {
		return err
	}
	return nil
}

func readString(r *bytes.Buffer) (string, error) {
	str, err := r.ReadString(0) //read filename
	if err != nil {
		return "", err
	}

	str = strings.TrimRight(str, "\x00") // remove the 0-byte
	if len(str) == 0 {
		return "", err
	}
	return str, nil
}
