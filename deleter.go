package main

import (
	"os"
	"sync"
	"time"
)

// DeleteItem 待删除项
type DeleteItem struct {
	Path      string
	DeleteAt  time.Time
	RetryLeft int
}

// DelayedDeleter 延迟删除管理器
type DelayedDeleter struct {
	mu         sync.Mutex
	queue      map[string]*DeleteItem
	retainHour int
	maxRetries int
	stopCh     chan struct{}
	logFn      func(string, ...interface{})
}

// NewDelayedDeleter 创建延迟删除管理器
func NewDelayedDeleter(retainHour int, logFn func(string, ...interface{})) *DelayedDeleter {
	d := &DelayedDeleter{
		queue:      make(map[string]*DeleteItem),
		retainHour: retainHour,
		maxRetries: 3,
		stopCh:     make(chan struct{}),
		logFn:      logFn,
	}
	go d.loop()
	return d
}

// Add 添加文件到删除队列
func (d *DelayedDeleter) Add(filePath string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	deleteTime := time.Now().Add(time.Duration(d.retainHour) * time.Hour)
	d.queue[filePath] = &DeleteItem{
		Path:      filePath,
		DeleteAt:  deleteTime,
		RetryLeft: d.maxRetries,
	}
	d.logFn("[INFO] [加入删除队列] 源文件将在 %s 删除", deleteTime.Format("2006-01-02 15:04:05"))
}

// Stop 停止删除循环
func (d *DelayedDeleter) Stop() {
	close(d.stopCh)
}

func (d *DelayedDeleter) loop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.processQueue()
		case <-d.stopCh:
			return
		}
	}
}

func (d *DelayedDeleter) processQueue() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	for path, item := range d.queue {
		if now.Before(item.DeleteAt) {
			continue
		}
		if err := os.Remove(path); err != nil {
			item.RetryLeft--
			if item.RetryLeft <= 0 {
				d.logFn("[ERROR] [删除失败] 已达到最大重试次数，放弃删除: %s, 错误: %v", path, err)
				delete(d.queue, path)
			} else {
				d.logFn("[WARN] [删除重试] 文件被占用: %s, 剩余重试次数: %d", path, item.RetryLeft)
			}
		} else {
			d.logFn("[INFO] [物理删除] 已删除源文件: %s", path)
			delete(d.queue, path)
		}
	}
}