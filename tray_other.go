//go:build !windows

package main

// 非 Windows 平台：runTray 是 no-op，不阻塞。
// 这样 main.go 在 Linux/macOS 也能编译通过（虽然没托盘）。
func runTray(onExit func()) {
	// 保持主进程存活
	if onExit != nil {
		// Linux 上没有托盘，main 仍走原来的 select{}
		// 此处不调用 onExit；调用方应继续 select{} 阻塞
	}
	select {}
}
