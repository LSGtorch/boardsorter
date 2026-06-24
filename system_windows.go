//go:build windows

package main

// system_windows.go 实现 boardsorter 在 Windows 平台上的系统集成：
//   1. 开机自启动（HKCU\Software\Microsoft\Windows\CurrentVersion\Run）
//   2. 开始菜单快捷方式（IShellLinkW + IPersistFile, COM via ole32.CoCreateInstance）
//
// 全部使用 syscall 直接调用 Windows API，不依赖任何第三方包。

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// =============================================================================
// 常量与类型
// =============================================================================

// 注册表访问权限
const (
	KEY_QUERY_VALUE = 0x0001
	KEY_SET_VALUE   = 0x0002
	KEY_READ        = 0x20019
	REG_SZ          = 1

	ERROR_SUCCESS        = 0
	ERROR_FILE_NOT_FOUND = 2
	ERROR_MORE_DATA      = 234
)

// 注册表根键
type HKEY uintptr

const (
	HKEY_CLASSES_ROOT   HKEY = 0x80000000
	HKEY_CURRENT_USER   HKEY = 0x80000001
	HKEY_LOCAL_MACHINE  HKEY = 0x80000002
	HKEY_USERS          HKEY = 0x80000003
	HKEY_CURRENT_CONFIG HKEY = 0x80000005
)

// COM 初始化标志
const (
	CLSCTX_INPROC_SERVER     = 0x1
	COINIT_APARTMENTTHREADED = 0x2
)

// GUID —— 与 Windows 布局完全一致
type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// CLSID_ShellLink = {00021401-0000-0000-C000-000000000046}
var CLSID_ShellLink = GUID{
	0x00021401, 0x0000, 0x0000,
	[8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
}

// IID_IShellLinkW = {000214F9-0000-0000-C000-000000000046}
var IID_IShellLinkW = GUID{
	0x000214F9, 0x0000, 0x0000,
	[8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
}

// IID_IPersistFile = {0000010B-0000-0000-C000-000000000046}
var IID_IPersistFile = GUID{
	0x0000010B, 0x0000, 0x0000,
	[8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
}

// =============================================================================
// DLL 与过程句柄
// =============================================================================

var (
	modAdvapi32 = syscall.NewLazyDLL("Advapi32.dll")
	modOle32    = syscall.NewLazyDLL("Ole32.dll")

	procRegOpenKeyExW    = modAdvapi32.NewProc("RegOpenKeyExW")
	procRegSetValueExW   = modAdvapi32.NewProc("RegSetValueExW")
	procRegQueryValueExW = modAdvapi32.NewProc("RegQueryValueExW")
	procRegCloseKey      = modAdvapi32.NewProc("RegCloseKey")
	procRegDeleteValueW  = modAdvapi32.NewProc("RegDeleteValueW")

	procCoCreateInstance = modOle32.NewProc("CoCreateInstance")
	procCoInitializeEx  = modOle32.NewProc("CoInitializeEx")
	procCoUninitialize  = modOle32.NewProc("CoUninitialize")
)

// =============================================================================
// 注册表辅助函数
// =============================================================================

func regOpenKeyEx(hKey HKEY, subKey string, sam uint32) (HKEY, error) {
	var result HKEY
	subKeyPtr, _ := syscall.UTF16PtrFromString(subKey)
	ret, _, _ := procRegOpenKeyExW.Call(
		uintptr(hKey),
		uintptr(unsafe.Pointer(subKeyPtr)),
		0,
		uintptr(sam),
		uintptr(unsafe.Pointer(&result)),
	)
	if ret != ERROR_SUCCESS {
		return 0, fmt.Errorf("RegOpenKeyExW failed: code=%d", ret)
	}
	return result, nil
}

func regCloseKey(hKey HKEY) error {
	ret, _, _ := procRegCloseKey.Call(uintptr(hKey))
	if ret != ERROR_SUCCESS {
		return fmt.Errorf("RegCloseKey failed: code=%d", ret)
	}
	return nil
}

func regSetStringValue(hKey HKEY, valueName, data string) error {
	valueNamePtr, _ := syscall.UTF16PtrFromString(valueName)
	dataUTF16, _ := syscall.UTF16FromString(data)
	// RegSetValueExW 期望的 cbData 包含末尾的 NUL 终止符（字节数）
	dataSize := uint32(len(dataUTF16) * 2)
	ret, _, _ := procRegSetValueExW.Call(
		uintptr(hKey),
		uintptr(unsafe.Pointer(valueNamePtr)),
		0,
		uintptr(REG_SZ),
		uintptr(unsafe.Pointer(&dataUTF16[0])),
		uintptr(dataSize),
	)
	if ret != ERROR_SUCCESS {
		return fmt.Errorf("RegSetValueExW failed: code=%d", ret)
	}
	return nil
}

// regQueryStringValue 返回 (值, Windows 错误码, error)。Windows 错误码用于
// 区分"值不存在"（ERROR_FILE_NOT_FOUND）等场景。
func regQueryStringValue(hKey HKEY, valueName string) (string, uint32, error) {
	valueNamePtr, _ := syscall.UTF16PtrFromString(valueName)
	const initialBufBytes uint32 = 2048
	dataSize := initialBufBytes
	data := make([]uint16, dataSize/2)
	ret, _, _ := procRegQueryValueExW.Call(
		uintptr(hKey),
		uintptr(unsafe.Pointer(valueNamePtr)),
		0,
		0,
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(unsafe.Pointer(&dataSize)),
	)
	if ret == ERROR_MORE_DATA {
		// 重新分配足够大的缓冲区（+1 防止 NUL 终止符边界问题）
		data = make([]uint16, dataSize/2+1)
		ret, _, _ = procRegQueryValueExW.Call(
			uintptr(hKey),
			uintptr(unsafe.Pointer(valueNamePtr)),
			0,
			0,
			uintptr(unsafe.Pointer(&data[0])),
			uintptr(unsafe.Pointer(&dataSize)),
		)
	}
	if ret != ERROR_SUCCESS {
		return "", uint32(ret), fmt.Errorf("RegQueryValueExW failed: code=%d", ret)
	}
	return syscall.UTF16ToString(data), uint32(ret), nil
}

func regDeleteValue(hKey HKEY, valueName string) error {
	valueNamePtr, _ := syscall.UTF16PtrFromString(valueName)
	ret, _, _ := procRegDeleteValueW.Call(
		uintptr(hKey),
		uintptr(unsafe.Pointer(valueNamePtr)),
	)
	if ret != ERROR_SUCCESS {
		return fmt.Errorf("RegDeleteValueW failed: code=%d", ret)
	}
	return nil
}

// =============================================================================
// 1. 开机自启动
// =============================================================================

const (
	autoStartKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	autoStartValueName = "boardsorter"
)

// SetAutoStart 启用或禁用 HKCU\...\Run 中的开机启动项。
// enabled=false 时删除已存在的值；enabled=true 时把 exePath 写入注册表。
func SetAutoStart(enabled bool, exePath string) error {
	hKey, err := regOpenKeyEx(HKEY_CURRENT_USER, autoStartKeyPath, KEY_QUERY_VALUE|KEY_SET_VALUE)
	if err != nil {
		return err
	}
	defer regCloseKey(hKey)

	if !enabled {
		return regDeleteValue(hKey, autoStartValueName)
	}
	return regSetStringValue(hKey, autoStartValueName, exePath)
}

// IsAutoStartEnabled 查询自启动项是否已启用。
func IsAutoStartEnabled() (bool, error) {
	hKey, err := regOpenKeyEx(HKEY_CURRENT_USER, autoStartKeyPath, KEY_QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer regCloseKey(hKey)

	_, code, err := regQueryStringValue(hKey, autoStartValueName)
	if err != nil {
		if code == ERROR_FILE_NOT_FOUND {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetAutoStartExePath 读取自启动项中保存的可执行文件路径。
func GetAutoStartExePath() (string, error) {
	hKey, err := regOpenKeyEx(HKEY_CURRENT_USER, autoStartKeyPath, KEY_QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer regCloseKey(hKey)
	value, _, err := regQueryStringValue(hKey, autoStartValueName)
	return value, err
}

// =============================================================================
// 2. COM 接口：IShellLinkW 与 IPersistFile
// =============================================================================

// IShellLinkWVtbl 按 Windows SDK 中的方法顺序排列。SetPath 在末尾。
type IShellLinkWVtbl struct {
	QueryInterface      uintptr // 0
	AddRef              uintptr // 1
	Release             uintptr // 2
	GetPath             uintptr // 3
	GetIDList           uintptr // 4
	SetIDList           uintptr // 5
	GetDescription      uintptr // 6
	SetDescription      uintptr // 7
	GetWorkingDirectory uintptr // 8
	SetWorkingDirectory uintptr // 9
	GetArguments        uintptr // 10
	SetArguments        uintptr // 11
	GetHotkey           uintptr // 12
	SetHotkey           uintptr // 13
	GetShowCmd          uintptr // 14
	SetShowCmd          uintptr // 15
	GetIconLocation     uintptr // 16
	SetIconLocation     uintptr // 17
	SetRelativePath     uintptr // 18
	Resolve             uintptr // 19
	SetPath             uintptr // 20
}

type IShellLinkW struct {
	Vtbl *IShellLinkWVtbl
}

type IPersistFileVtbl struct {
	QueryInterface uintptr // 0
	AddRef         uintptr // 1
	Release        uintptr // 2
	GetClassID     uintptr // 3
	IsDirty        uintptr // 4
	Load           uintptr // 5
	Save           uintptr // 6
	SaveCompleted  uintptr // 7
	GetCurFile     uintptr // 8
}

type IPersistFile struct {
	Vtbl *IPersistFileVtbl
}

// ----- IShellLinkW 方法包装 -----

func (sl *IShellLinkW) Release() {
	syscall.SyscallN(sl.Vtbl.Release, uintptr(unsafe.Pointer(sl)))
}

func (sl *IShellLinkW) SetPath(path string) error {
	pathPtr, _ := syscall.UTF16PtrFromString(path)
	ret, _, _ := syscall.SyscallN(
		sl.Vtbl.SetPath,
		uintptr(unsafe.Pointer(sl)),
		uintptr(unsafe.Pointer(pathPtr)),
	)
	if ret != 0 {
		return fmt.Errorf("IShellLinkW::SetPath failed: 0x%x", ret)
	}
	return nil
}

func (sl *IShellLinkW) SetWorkingDirectory(dir string) error {
	dirPtr, _ := syscall.UTF16PtrFromString(dir)
	ret, _, _ := syscall.SyscallN(
		sl.Vtbl.SetWorkingDirectory,
		uintptr(unsafe.Pointer(sl)),
		uintptr(unsafe.Pointer(dirPtr)),
	)
	if ret != 0 {
		return fmt.Errorf("IShellLinkW::SetWorkingDirectory failed: 0x%x", ret)
	}
	return nil
}

func (sl *IShellLinkW) SetDescription(desc string) error {
	descPtr, _ := syscall.UTF16PtrFromString(desc)
	ret, _, _ := syscall.SyscallN(
		sl.Vtbl.SetDescription,
		uintptr(unsafe.Pointer(sl)),
		uintptr(unsafe.Pointer(descPtr)),
	)
	if ret != 0 {
		return fmt.Errorf("IShellLinkW::SetDescription failed: 0x%x", ret)
	}
	return nil
}

// ----- IPersistFile 方法包装 -----

func (pf *IPersistFile) Release() {
	syscall.SyscallN(pf.Vtbl.Release, uintptr(unsafe.Pointer(pf)))
}

func (pf *IPersistFile) Save(filePath string, fRemember bool) error {
	pathPtr, _ := syscall.UTF16PtrFromString(filePath)
	var bRemember uintptr
	if fRemember {
		bRemember = 1
	}
	ret, _, _ := syscall.SyscallN(
		pf.Vtbl.Save,
		uintptr(unsafe.Pointer(pf)),
		uintptr(unsafe.Pointer(pathPtr)),
		bRemember,
	)
	if ret != 0 {
		return fmt.Errorf("IPersistFile::Save failed: 0x%x", ret)
	}
	return nil
}

// =============================================================================
// COM 生命周期管理
// =============================================================================

// coInitialize 在当前线程上初始化 COM。S_OK(0) 与 S_FALSE(1) 都视为成功。
func coInitialize() error {
	ret, _, _ := procCoInitializeEx.Call(0, COINIT_APARTMENTTHREADED)
	if ret != 0 && ret != 1 {
		return fmt.Errorf("CoInitializeEx failed: 0x%x", ret)
	}
	return nil
}

func coUninitialize() {
	procCoUninitialize.Call()
}

// createShortcut 通过 IShellLinkW + IPersistFile 创建 .lnk 快捷方式。
func createShortcut(exePath, linkPath, description string) error {
	if err := coInitialize(); err != nil {
		return err
	}
	defer coUninitialize()

	// CoCreateInstance(CLSID_ShellLink, NULL, CLSCTX_INPROC_SERVER, IID_IShellLinkW, &pShellLink)
	var pShellLink unsafe.Pointer
	ret, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&CLSID_ShellLink)),
		0,
		uintptr(CLSCTX_INPROC_SERVER),
		uintptr(unsafe.Pointer(&IID_IShellLinkW)),
		uintptr(unsafe.Pointer(&pShellLink)),
	)
	if ret != 0 || pShellLink == nil {
		return fmt.Errorf("CoCreateInstance(IShellLinkW) failed: 0x%x", ret)
	}
	sl := (*IShellLinkW)(pShellLink)
	defer sl.Release()

	if err := sl.SetPath(exePath); err != nil {
		return err
	}
	if err := sl.SetWorkingDirectory(filepath.Dir(exePath)); err != nil {
		return err
	}
	if err := sl.SetDescription(description); err != nil {
		return err
	}

	// QueryInterface(IID_IPersistFile)
	var pPersistFile unsafe.Pointer
	ret, _, _ = syscall.SyscallN(
		sl.Vtbl.QueryInterface,
		uintptr(unsafe.Pointer(sl)),
		uintptr(unsafe.Pointer(&IID_IPersistFile)),
		uintptr(unsafe.Pointer(&pPersistFile)),
	)
	if ret != 0 || pPersistFile == nil {
		return fmt.Errorf("QueryInterface(IPersistFile) failed: 0x%x", ret)
	}
	pf := (*IPersistFile)(pPersistFile)
	defer pf.Release()

	return pf.Save(linkPath, true)
}

// =============================================================================
// 3. 开始菜单快捷方式
// =============================================================================

// getStartMenuProgramsPath 返回 %APPDATA%\Microsoft\Windows\Start Menu\Programs
func getStartMenuProgramsPath() (string, error) {
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		return "", errors.New("APPDATA environment variable is not set")
	}
	return filepath.Join(appdata, "Microsoft", "Windows", "Start Menu", "Programs"), nil
}

// getStartMenuStartUpPath 返回 %APPDATA%\Microsoft\Windows\Start Menu\Programs\StartUp
func getStartMenuStartUpPath() (string, error) {
	programs, err := getStartMenuProgramsPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(programs, "StartUp"), nil
}

// CreateStartMenuShortcuts 在开始菜单的 Programs 与 StartUp 下创建 boardsorter.lnk。
func CreateStartMenuShortcuts(exePath, appName string) error {
	programsPath, err := getStartMenuProgramsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(programsPath, 0755); err != nil {
		return fmt.Errorf("failed to create programs directory %q: %w", programsPath, err)
	}
	if err := createShortcut(exePath, filepath.Join(programsPath, appName+".lnk"), appName); err != nil {
		return fmt.Errorf("failed to create programs shortcut: %w", err)
	}

	startupPath, err := getStartMenuStartUpPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(startupPath, 0755); err != nil {
		return fmt.Errorf("failed to create startup directory %q: %w", startupPath, err)
	}
	if err := createShortcut(exePath, filepath.Join(startupPath, appName+".lnk"), appName); err != nil {
		return fmt.Errorf("failed to create startup shortcut: %w", err)
	}
	return nil
}

// RemoveStartMenuShortcuts 删除开始菜单 Programs 与 StartUp 下的 boardsorter.lnk。
func RemoveStartMenuShortcuts(appName string) error {
	programsPath, err := getStartMenuProgramsPath()
	if err != nil {
		return err
	}
	mainLink := filepath.Join(programsPath, appName+".lnk")
	if err := os.Remove(mainLink); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove programs shortcut: %w", err)
	}

	startupPath, err := getStartMenuStartUpPath()
	if err != nil {
		return err
	}
	startupLink := filepath.Join(startupPath, appName+".lnk")
	if err := os.Remove(startupLink); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove startup shortcut: %w", err)
	}
	return nil
}
