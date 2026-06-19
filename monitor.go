package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Handler 文件事件处理器
type Handler func(filePath string)

// Monitor 文件监控器
type Monitor struct {
	watchDir string
	watcher  *fsnotify.Watcher
	handler  Handler
	stopCh   chan struct{}
	logFn    func(string, ...interface{})
}

// NewMonitor 创建文件监控器
func NewMonitor(watchDir string, handler Handler, logFn func(string, ...interface{})) (*Monitor, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := w.Add(watchDir); err != nil {
		w.Close()
		return nil, err
	}

	m := &Monitor{
		watchDir: watchDir,
		watcher:  w,
		handler:  handler,
		stopCh:   make(chan struct{}),
		logFn:    logFn,
	}
	return m, nil
}

// Start 开始监控
func (m *Monitor) Start() {
	go m.loop()
}

func (m *Monitor) loop() {
	// 用于去重，防止同一文件多次触发
	processed := make(map[string]time.Time)

	for {
		select {
		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}
			// 只处理创建和写入事件
			if event.Op&(fsnotify.Create|fsnotify.Write) == 0 {
				continue
			}
			// 忽略临时文件和隐藏文件
			base := filepath.Base(event.Name)
			if strings.HasPrefix(base, "~") || strings.HasPrefix(base, ".") {
				continue
			}

			// 去重（5秒内同一文件不重复处理）
			if last, ok := processed[event.Name]; ok {
				if time.Since(last) < 5*time.Second {
					continue
				}
			}
			processed[event.Name] = time.Now()

			// 稍作等待，确保文件写入完成
			time.Sleep(2 * time.Second)

			// 检查文件是否存在并可读
			if !m.isFileReady(event.Name) {
				continue
			}

			m.logFn("[INFO] [新文件] 检测到: %s", filepath.Base(event.Name))
			m.handler(event.Name)

		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			m.logFn("[WARN] 监控错误: %v", err)

		case <-m.stopCh:
			return
		}
	}
}

// Stop 停止监控
func (m *Monitor) Stop() {
	close(m.stopCh)
	m.watcher.Close()
}

// isFileReady 检查文件是否可读（写入完成）
func (m *Monitor) isFileReady(path string) bool {
	// 检查文件是否存在
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	// 检查是否为空文件
	if info.Size() == 0 {
		return false
	}
	// 尝试以独占方式打开
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}
