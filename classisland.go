package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// ClassIslandNotifier 分类完成时向 ClassIsland 发送通知
type ClassIslandNotifier struct {
	enabled    bool
	apiURL     string
	template   string
	httpClient *http.Client
	logFn      func(string, ...interface{})
}

// NewClassIslandNotifier 创建通知器
func NewClassIslandNotifier(enabled bool, apiURL, template string, logFn func(string, ...interface{})) *ClassIslandNotifier {
	if apiURL == "" {
		apiURL = "http://localhost:5000"
	}
	if template == "" {
		template = "📁 {filename} → {subject}"
	}
	return &ClassIslandNotifier{
		enabled:  enabled,
		apiURL:   strings.TrimRight(apiURL, "/"),
		template: template,
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
		logFn: logFn,
	}
}

// Notify 发送分类通知
func (n *ClassIslandNotifier) Notify(filePath, subject, category string) {
	if !n.enabled {
		return
	}

	fileName := filepath.Base(filePath)
	msg := strings.ReplaceAll(n.template, "{filename}", fileName)
	msg = strings.ReplaceAll(msg, "{subject}", subject)
	msg = strings.ReplaceAll(msg, "{category}", category)

	// ClassIsland 通知 API 格式
	body := map[string]string{
		"title":   "boardsorter 文件分类",
		"content": msg,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return
	}

	go func() {
		resp, err := n.httpClient.Post(
			n.apiURL+"/api/notification",
			"application/json",
			bytes.NewReader(jsonBody),
		)
		if err != nil {
			if n.logFn != nil {
				n.logFn("[DEBUG] ClassIsland 通知发送失败: %v", err)
			}
			return
		}
		resp.Body.Close()
		if n.logFn != nil {
			n.logFn("[INFO] ClassIsland 通知已发送: %s", msg)
		}
	}()
}

// IsEnabled 返回通知是否启用
func (n *ClassIslandNotifier) IsEnabled() bool {
	return n.enabled
}