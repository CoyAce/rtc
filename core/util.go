package core

import (
	"log"
	"os"
	"strings"
)

func GetDir(uuid string) string {
	return strings.Replace(uuid, "#", "_", -1)
}

func GetFileName(uuid string, filename string) string {
	if uuid == "" {
		return filename
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

func mkdir(dir string) {
	if len(dir) == 0 {
		return
	}
	// 使用 MkdirAll 确保目录存在
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		log.Fatalf("Error creating directory: %v", err)
	}
}
