package main

import (
	"os"
	"path/filepath"
	"strings"
)

// listFiles 列出目录下所有普通文件（不递归子目录）
func listFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// 跳过隐藏文件
		if strings.HasPrefix(name, ".") {
			continue
		}
		// 跳过临时文件
		if strings.HasPrefix(name, "~") || strings.HasSuffix(name, ".tmp") {
			continue
		}
		result = append(result, filepath.Join(dir, name))
	}
	return result, nil
}
