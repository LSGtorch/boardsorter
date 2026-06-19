package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Archiver 归档执行器
type Archiver struct {
	logFn func(string, ...interface{})
}

// NewArchiver 创建归档执行器
func NewArchiver(logFn func(string, ...interface{})) *Archiver {
	return &Archiver{logFn: logFn}
}

// Archive 将文件复制到目标目录，返回目标路径
func (a *Archiver) Archive(srcPath, destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("创建目标目录失败: %w", err)
	}

	destPath := filepath.Join(destDir, filepath.Base(srcPath))
	destPath = a.resolveConflict(destPath)

	if err := copyFile(srcPath, destPath); err != nil {
		return "", fmt.Errorf("复制文件失败: %w", err)
	}

	a.logFn("[INFO] [归档成功] %s -> %s", filepath.Base(srcPath), destPath)
	return destPath, nil
}

// resolveConflict 处理同名文件冲突
func (a *Archiver) resolveConflict(destPath string) string {
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return destPath
	}
	ext := filepath.Ext(destPath)
	base := destPath[:len(destPath)-len(ext)]
	timestamp := time.Now().Format("20060102_150405")
	return fmt.Sprintf("%s_%s%s", base, timestamp, ext)
}

func copyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return fmt.Errorf("复制内容失败: %w", err)
	}
	return nil
}