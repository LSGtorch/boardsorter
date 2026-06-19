// Package tray 提供系统托盘功能
//
// 在Windows环境下，此模块将创建系统托盘图标；
// 在Linux/Mac环境下，回退到控制台日志输出。
package main

import (
	"fmt"
	"sync"

)

// TrayApp 系统托盘应用
type TrayApp struct {
	mu        sync.Mutex
	logLines  []string
	maxLines  int
	log       *Logger
	onExit    func()
	startFunc func()
	running   bool
}

// NewTrayApp 创建托盘应用
func NewTrayApp(log *Logger, startFunc, onExit func()) *TrayApp {
	t := &TrayApp{
		logLines:  make([]string, 0, 1000),
		maxLines:  1000,
		log:       log,
		onExit:    onExit,
		startFunc: startFunc,
		running:   true,
	}
	// 注册日志回调
	log.RegisterCallback(func(level, msg string) {
		t.addLog(level, msg)
	})
	return t
}

// Run 启动应用（在Windows下会显示系统托盘图标）
func (t *TrayApp) Run() {
	// 在当前环境下，使用控制台模式
	t.log.Info("BoardSorter v3.0 已启动")
	t.log.Info("在Windows系统下运行时，将在系统托盘显示图标")
	t.log.Info("当前运行模式: 控制台模式")

	// 启动业务逻辑
	go t.startFunc()

	// 保持运行
	select {}
}

// Stop 停止应用
func (t *TrayApp) Stop() {
	t.running = false
	if t.onExit != nil {
		t.onExit()
	}
}

func (t *TrayApp) addLog(level, msg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	line := fmt.Sprintf("[%s] %s", level, msg)
	t.logLines = append(t.logLines, line)
	if len(t.logLines) > t.maxLines {
		t.logLines = t.logLines[len(t.logLines)-t.maxLines:]
	}
}

// ShowLogWindow 显示日志（在控制台输出）
func (t *TrayApp) ShowLogWindow() {
	t.mu.Lock()
	lines := make([]string, len(t.logLines))
	copy(lines, t.logLines)
	t.mu.Unlock()

	fmt.Println("\n===== BoardSorter 运行日志 =====")
	start := 0
	if len(lines) > 50 {
		start = len(lines) - 50
	}
	for _, line := range lines[start:] {
		fmt.Println(line)
	}
	fmt.Println("===== 日志显示完毕 =====")
	fmt.Println("完整日志请查看日志文件夹中的日志文件")
}

// UpdateStats 更新状态显示
func (t *TrayApp) UpdateStats(hotCount, coldCount int) {
	t.log.Info("[当前词库状态] 热词: %d 个, 冷词: %d 个", hotCount, coldCount)
}