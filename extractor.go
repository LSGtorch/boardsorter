package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

const maxContentLen = 3000

// Extractor 文件内容提取器
type Extractor struct {
	readableExts []string
	archiveExts  []string
}

// NewExtractor 创建提取器
func NewExtractor(readableExts, archiveExts []string) *Extractor {
	return &Extractor{
		readableExts: readableExts,
		archiveExts:  archiveExts,
	}
}

// CanExtract 判断文件是否可以被提取内容
func (e *Extractor) CanExtract(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, re := range e.readableExts {
		if ext == re {
			return true
		}
	}
	for _, ae := range e.archiveExts {
		if ext == ae {
			return true
		}
	}
	return false
}

// Extract 提取文件内容，返回前N个字符
func (e *Extractor) Extract(filePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".txt":
		return e.extractTxt(filePath)
	case ".docx":
		return e.extractDocx(filePath)
	case ".pptx":
		return e.extractPptx(filePath)
	case ".pdf":
		return e.extractPdf(filePath)
	case ".zip":
		return e.extractZip(filePath)
	default:
		return "", fmt.Errorf("不支持的文件类型: %s", ext)
	}
}

func (e *Extractor) extractTxt(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("读取txt失败: %w", err)
	}
	content := string(data)
	if len([]rune(content)) > maxContentLen {
		content = string([]rune(content)[:maxContentLen])
	}
	return content, nil
}

func (e *Extractor) extractDocx(filePath string) (string, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("打开docx失败: %w", err)
	}
	defer r.Close()

	var textBuf bytes.Buffer
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			defer rc.Close()
			data, _ := io.ReadAll(rc)
			// 简单的XML标签去除
			content := stripXMLTags(string(data))
			textBuf.WriteString(content)
			if textBuf.Len() >= maxContentLen {
				break
			}
		}
	}
	result := textBuf.String()
	if len([]rune(result)) > maxContentLen {
		result = string([]rune(result)[:maxContentLen])
	}
	if result == "" {
		return "", fmt.Errorf("docx未提取到文本")
	}
	return result, nil
}

func (e *Extractor) extractPptx(filePath string) (string, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("打开pptx失败: %w", err)
	}
	defer r.Close()

	var textBuf bytes.Buffer
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, _ := io.ReadAll(rc)
			content := stripXMLTags(string(data))
			textBuf.WriteString(content)
			textBuf.WriteString("\n")
			if textBuf.Len() >= maxContentLen {
				break
			}
			rc.Close()
		}
	}
	result := textBuf.String()
	if len([]rune(result)) > maxContentLen {
		result = string([]rune(result)[:maxContentLen])
	}
	if result == "" {
		return "", fmt.Errorf("pptx未提取到文本")
	}
	return result, nil
}

func (e *Extractor) extractPdf(filePath string) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开pdf失败: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	totalReader, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("读取pdf文本失败: %w", err)
	}

	_, err = io.CopyN(&buf, totalReader, int64(maxContentLen)*3) // 多读一些，后面截断
	if err != nil && err != io.EOF {
		// CopyN 在读到 n 之前遇到 EOF 会返回 error
		// 所以需要容忍 EOF
	}
	content := buf.String()
	if len([]rune(content)) > maxContentLen {
		content = string([]rune(content)[:maxContentLen])
	}
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("pdf未提取到文本")
	}
	return content, nil
}

func (e *Extractor) extractZip(filePath string) (string, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("打开压缩包失败: %w", err)
	}
	defer r.Close()

	var textBuf bytes.Buffer
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(f.Name))
		// 只提取文本类文件
		if ext != ".txt" && ext != ".docx" && ext != ".md" && ext != ".csv" && ext != ".json" && ext != ".xml" && ext != ".html" && ext != ".htm" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		textBuf.WriteString(fmt.Sprintf("[%s]: ", filepath.Base(f.Name)))
		if ext == ".docx" {
			// 简单docx提取
			zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err == nil {
				for _, zf := range zr.File {
					if zf.Name == "word/document.xml" {
						rc2, _ := zf.Open()
						d, _ := io.ReadAll(rc2)
						rc2.Close()
						textBuf.WriteString(stripXMLTags(string(d)))
					}
				}
			}
		} else {
			textBuf.WriteString(string(data))
		}
		textBuf.WriteString("\n")
		if textBuf.Len() >= maxContentLen {
			break
		}
	}
	result := textBuf.String()
	if len([]rune(result)) > maxContentLen {
		result = string([]rune(result)[:maxContentLen])
	}
	if result == "" {
		return "", fmt.Errorf("压缩包未提取到可读文本")
	}
	return result, nil
}

// stripXMLTags 简单的XML标签去除
func stripXMLTags(s string) string {
	var buf bytes.Buffer
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			buf.WriteRune(r)
		}
	}
	return strings.TrimSpace(buf.String())
}