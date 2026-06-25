package main

import (
	"sync"
	"time"
)

// ClassIslandNotifier 管理分类通知队列，供 GUI 轮询
type ClassIslandNotifier struct {
	mu       sync.Mutex
	enabled  bool
	queue    []ClassIslandNotification
	maxSize  int
}

// ClassIslandNotification 单条分类通知
type ClassIslandNotification struct {
	Time     string `json:"time"`
	FileName string `json:"file_name"`
	Subject  string `json:"subject"`
}

// NewClassIslandNotifier 创建通知器
func NewClassIslandNotifier(enabled bool) *ClassIslandNotifier {
	return &ClassIslandNotifier{
		enabled: enabled,
		queue:   make([]ClassIslandNotification, 0, 64),
		maxSize: 100,
	}
}

// SetEnabled 设置启用状态
func (n *ClassIslandNotifier) SetEnabled(enabled bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.enabled = enabled
}

// IsEnabled 返回是否启用
func (n *ClassIslandNotifier) IsEnabled() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.enabled
}

// Notify 添加一条分类通知到队列
func (n *ClassIslandNotifier) Notify(fileName, subject string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.enabled {
		return
	}
	n.queue = append(n.queue, ClassIslandNotification{
		Time:     time.Now().Format("15:04:05"),
		FileName: fileName,
		Subject:  subject,
	})
	if len(n.queue) > n.maxSize {
		n.queue = n.queue[len(n.queue)-n.maxSize:]
	}
}

// Drain 取出所有待发送通知并清空队列
func (n *ClassIslandNotifier) Drain() []ClassIslandNotification {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.queue) == 0 {
		return nil
	}
	result := make([]ClassIslandNotification, len(n.queue))
	copy(result, n.queue)
	n.queue = n.queue[:0]
	return result
}