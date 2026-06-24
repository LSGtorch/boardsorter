//go:build !windows

package main

import "fmt"

// system_other.go 为非 Windows 平台提供 system_windows.go 中同名的桩实现，
// 以便 boardsorter 可以在 Linux/macOS 等平台上正常编译与运行。
// 所有函数都直接返回 nil 或 (false, nil)，不进行任何实际的系统修改。

// SetAutoStart 在非 Windows 平台上为 no-op。
func SetAutoStart(enabled bool, exePath string) error {
	return nil
}

// IsAutoStartEnabled 在非 Windows 平台上始终返回 false。
func IsAutoStartEnabled() (bool, error) {
	return false, nil
}

// GetAutoStartExePath 在非 Windows 平台上始终返回空字符串。
func GetAutoStartExePath() (string, error) {
	return "", nil
}

// CreateStartMenuShortcuts 在非 Windows 平台上为 no-op。
func CreateStartMenuShortcuts(exePath, appName string) error {
	return nil
}

// RemoveStartMenuShortcuts 在非 Windows 平台上为 no-op。
func RemoveStartMenuShortcuts(appName string) error {
	return nil
}

// getStartMenuProgramsPath 在非 Windows 平台上始终返回错误。
func getStartMenuProgramsPath() (string, error) {
	return "", errNotWindows
}

// getStartMenuStartUpPath 在非 Windows 平台上始终返回错误。
func getStartMenuStartUpPath() (string, error) {
	return "", errNotWindows
}

var errNotWindows = fmt.Errorf("system integration is only supported on Windows")
