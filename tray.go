package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/getlantern/systray"
)

// TrayApp 系统托盘应用
type TrayApp struct {
	mu        sync.Mutex
	logLines  []string
	maxLines  int
	onExit    func()
	startFunc func()
	running   bool
}

// iconDataBase64 是托盘图标（16x16 蓝色方块 .ico，base64 编码）
var iconDataBase64 = "AAABAAEAEBAAAAEAIABoBAAAFgAAACgAAAAQAAAAIAAAAAEAIAAAAAAAQAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD/mjL/ZTQo/2U0KP9lNCj/ZTQo/2U0KP9lNCj/ZTQo/2U0KP9lNCj/ZTQo/2U0KP9lNCj/ZTQo/2U0KP9lNCj/ZTQo/5oy/wAAAAAAAAAA/5oy/2U0KP9lNCj/ZTQo/2U0KP9lNCj/ZTQo/2U0KP9lNCj/ZTQo/2U0KP9lNCj/ZTQo/2U0KP9lNCj/ZTQo/5oy/wAAAAAAAAAAAAAAAP+aMv9lNCj/ZTQo/2U0KP9lNCj/ZTQo/2U0KP9lNCj/ZTQo/2U0KP9lNCj/ZTQo/2U0KP9lNCj/ZTQo/5oy/wAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="

// iconDataRaw 解码后的图标原始数据
var iconDataRaw []byte

func init() {
	data, _ := base64.StdEncoding.DecodeString(iconDataBase64)
	iconDataRaw = data
}

// NewTrayApp 创建托盘应用
func NewTrayApp(startFunc, onExit func()) *TrayApp {
	return &TrayApp{
		logLines:  make([]string, 0, 1000),
		maxLines:  1000,
		onExit:    onExit,
		startFunc: startFunc,
		running:   true,
	}
}

// Run 启动系统托盘（阻塞，直到调用 Stop 或用户退出）
func (t *TrayApp) Run() {
	systray.SetIcon(iconDataRaw)
	systray.SetTitle("BoardSorter")
	systray.SetTooltip("BoardSorter - 教学文件归档系统")

	menuShowLog := systray.AddMenuItem("查看日志", "在记事本显示运行日志")
	systray.AddSeparator()
	menuExit := systray.AddMenuItem("退出", "退出程序")

	go t.startFunc()

	for t.running {
		select {
		case <-menuShowLog.ClickedCh:
			t.showLogWindow()
		case <-menuExit.ClickedCh:
			t.Stop()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// Stop 停止托盘并退出
func (t *TrayApp) Stop() {
	t.mu.Lock()
	t.running = false
	t.mu.Unlock()
	systray.Quit()
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

// showLogWindow 在记事本显示日志
func (t *TrayApp) showLogWindow() {
	t.mu.Lock()
	lines := make([]string, len(t.logLines))
	copy(lines, t.logLines)
	t.mu.Unlock()

	tmpfile := os.TempDir() + string(os.PathSeparator) + "boardsorter_log.txt"
	var buf bytes.Buffer
	buf.WriteString("BoardSorter 运行日志 (最近 50 条)\n")
	buf.WriteString(fmt.Sprintf("查看时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	buf.WriteString("========================================\n\n")
	start := 0
	if len(lines) > 50 {
		start = len(lines) - 50
	}
	for _, line := range lines[start:] {
		buf.WriteString(line + "\n")
	}
	buf.WriteString("\n========================================\n")
	buf.WriteString(fmt.Sprintf("共 %d 条日志\n", len(lines)))

	os.WriteFile(tmpfile, []byte(buf.String()), 0644)
	if runtime.GOOS == "windows" {
		exec.Command("notepad.exe", tmpfile).Start()
	}
}

// GetRecentLogs 获取最近 N 条日志
func (t *TrayApp) GetRecentLogs(n int) []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.logLines) == 0 {
		return nil
	}
	start := 0
	if len(t.logLines) > n {
		start = len(t.logLines) - n
	}
	result := make([]string, len(t.logLines)-start)
	copy(result, t.logLines[start:])
	return result
}
