//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/getlantern/systray"
)

// trayOnExit 由 main.go 在启动时注入：当用户在托盘点"退出"时调用。
var trayOnExit func()

// runTray 启动系统托盘，阻塞当前 goroutine 直到 systray.Quit()。
// 调用方应保证 trayOnExit 已注入。
func runTray(onExit func()) {
	trayOnExit = onExit
	systray.Run(onTrayReady, onTrayExit)
}

func onTrayReady() {
	systray.SetTitle("Boardsorter")
	systray.SetTooltip("boardsorter 高中教学文件自动归档")

	mShow := systray.AddMenuItem("显示主界面", "打开 boardsorter-config.exe")
	mStatus := systray.AddMenuItem("运行状态", "查看监控/词条/文件数")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "关闭 boardsorter 服务")

	go func() {
		for {
			select {
			case <-mShow.ClickedCh:
				launchConfigUI()
			case <-mStatus.ClickedCh:
				showStatusNotification()
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func onTrayExit() {
	if trayOnExit != nil {
		trayOnExit()
	}
}

// launchConfigUI 同目录启动 boardsorter-config.exe
func launchConfigUI() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Dir(exePath)
	configPath := filepath.Join(dir, "boardsorter-config.exe")
	if _, err := os.Stat(configPath); err != nil {
		return
	}
	cmd := exec.Command(configPath)
	cmd.Dir = dir
	_ = cmd.Start()
}

// showStatusNotification 在托盘 tooltip 显示当前状态
func showStatusNotification() {
	var info string
	if ipcConfig != nil {
		info = "监控: " + ipcConfig.WatchFolder
	}
	if ipcMetadata != nil {
		total, pending := ipcMetadata.GetStats()
		info += " | 文件: " + itoa(total) + " 待确认: " + itoa(pending)
	}
	if info == "" {
		info = "boardsorter 运行中"
	}
	systray.SetTooltip(info)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
