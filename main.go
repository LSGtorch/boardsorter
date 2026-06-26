package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	configFileName = "config.ini"
	dataDirName    = "data"
	appDisplayName = "boardsorter"
	appVersion     = "1.3"
)

// 全局组件引用，供 startServer 使用
var (
	appLog        *Logger
	appTermDB     *TermDB
	appMetadata   *FileMetadata
	appDelDeleter *DelayedDeleter
	appMonitor    *Monitor
)

func main() {
	execDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取运行目录失败: %v\n", err)
		os.Exit(1)
	}

	// 检查命令行参数
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help", "-h":
			printHelp()
			return
		case "--no-ipc":
			// 调试模式：跳过 IPC 服务
			runConsole(execDir, true)
			return
		case "--cleanup-system":
			// 清理模式：删除自启动 + 快捷方式后退出
			cleanupSystem()
			return
		case "--console", "-c":
			runConsole(execDir, false)
			return
		}
	}

	// 默认控制台模式运行
	runConsole(execDir, false)
}

func runConsole(execDir string, noIPC bool) {
	cfg, log := initSystem(execDir)
	if cfg == nil || log == nil {
		// 只在有控制台时才等待输入（GUI 模式无控制台）
		if isConsoleAttached() {
			fmt.Println("按回车键退出...")
			fmt.Scanln()
		}
		os.Exit(1)
	}
	startServer(cfg, log, noIPC)

	// noIPC 模式（调试）保持原 select{} 阻塞；正常模式启动系统托盘阻塞。
	if noIPC {
		select {}
	}
	log.Info("服务运行中，按 Ctrl+C 退出")
	// 由于 Go 端无托盘，GUI 程序负责管理生命周期
	// 使用 select{} 保持进程存活；退出由 /api/stop 或外部信号控制
	select {}
}

func initSystem(execDir string) (*Config, *Logger) {
	// 寻找配置文件
	configPath := filepath.Join(execDir, configFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		execPath, _ := os.Executable()
		configPath = filepath.Join(filepath.Dir(execPath), configFileName)
	}

	// 加载配置
	cfg, err := LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		fmt.Fprintf(os.Stderr, "请检查配置文件 %s\n", configPath)
		return nil, nil
	}

	// 启动时自动追加缺失的配置项（向后兼容老用户）
	if err := appendMissingDefaults(configPath); err != nil {
		fmt.Fprintf(os.Stderr, "追加缺失配置项失败: %v\n", err)
	}

	// 确保目录存在
	if err := cfg.EnsureDirectories(); err != nil {
		fmt.Fprintf(os.Stderr, "创建目录失败: %v\n", err)
		return nil, nil
	}

	// 检查监控目录
	if _, err := os.Stat(cfg.WatchFolder); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "监控目录不存在: %s\n", cfg.WatchFolder)
		return nil, nil
	}

	// 初始化日志系统
	log, err := NewLogger(cfg.LogFolder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		return nil, nil
	}

	log.Info("========================================")
	log.Info("  %s v%s 启动", appDisplayName, appVersion)
	log.Info("  配置文件: %s", configPath)
	log.Info("  监控目录: %s", cfg.WatchFolder)
	log.Info("  归档目录: %s", cfg.ArchiveRoot)
	log.Info("  学科类目: %s", cfg.SubjectList())
	log.Info("========================================")

	return cfg, log
}

func startServer(cfg *Config, log *Logger, noIPC bool) {
	execDir, _ := os.Getwd()

	// 初始化数据存储
	dataDir := filepath.Join(execDir, dataDirName)

	// 词条数据库
	termDB, err := NewTermDB(dataDir, cfg.SubjectFolders, log.Raw)
	if err != nil {
		log.Error("初始化词条库失败: %v", err)
		return
	}
	appTermDB = termDB
	termCount := termDB.GetStats()
	log.Info("词条库加载完成: %d 个词条", termCount)

	// 文件元数据
	metadata, err := NewFileMetadata(dataDir)
	if err != nil {
		log.Error("初始化文件元数据失败: %v", err)
		return
	}
	appMetadata = metadata
	total, pending := metadata.GetStats()
	log.Info("文件元数据: 已记录 %d 个文件, 待确认 %d 个", total, pending)

	// 初始化AI客户端
	aiClient := NewAIClient(
		cfg.AIEndpoint,
		cfg.APIKey,
		cfg.ModelName,
		cfg.AIPrompt,
		cfg.RetryWaitSec,
		cfg.MaxRetries,
		cfg.ReasoningEffort,
	)

	// 初始化内容提取器
	extract := NewExtractor(cfg.ReadableExtList, cfg.ArchiveExtList)

	// 初始化归档执行器
	arch := NewArchiver(log.Raw)

	// 初始化延迟删除器
	delDeleter := NewDelayedDeleter(cfg.SourceRetainHour, log.Raw)
	appDelDeleter = delDeleter

	// 初始化分类器
	class := NewClassifier(
		cfg.SubjectFolders,
		cfg.ArchiveRoot,
		cfg.IrrelevantFolder,
		cfg.UncertainFolder,
		termDB,
		metadata,
		aiClient,
		extract,
		arch,
		delDeleter,
		log.Raw,
	)

	// 启动时扫描用户手动放入的文件
	class.ScanSubjectFolders(cfg.SubjectFolders)
	totalAfter, _ := metadata.GetStats()
	if totalAfter > total {
		log.Info("[启动扫描] 补录 %d 个手动文件", totalAfter-total)

	}

	// 启动文件监控
	mon, err := NewMonitor(cfg.WatchFolder, class.ClassifyFile, log.Raw)
	if err != nil {
		log.Error("启动文件监控失败: %v", err)
		return
	}
	mon.Start()
	appMonitor = mon
	log.Info("开始监控文件夹: %s", cfg.WatchFolder)

	// 定时任务：每小时扫描手动放入的文件 + 每日词条衰减
	go func() {
		// 首次扫描延迟1分钟（避免启动时与监控事件冲突）
		nextScan := time.Now().Add(1 * time.Minute)
		// 词条衰减每日一次
		nextDecay := time.Now().Add(24 * time.Hour)

		for {
			now := time.Now()
			sleepFor := time.Hour
			if nextScan.Before(now) {
				sleepFor = 1 * time.Minute
			} else {
				sleepFor = nextScan.Sub(now)
			}
			time.Sleep(sleepFor)

			now = time.Now()
			if now.After(nextScan) {
				log.Info("[定时扫描] 开始扫描用户手动放入的文件...")
				class.ScanSubjectFolders(cfg.SubjectFolders)
				nextScan = now.Add(1 * time.Hour)
			}
			if now.After(nextDecay) {
				log.Info("[定时任务] 开始词条衰减...")
				termDB.Decay(cfg.TermMaxIdleDays)
				nextDecay = now.Add(24 * time.Hour)
			}
		}
	}()

	// 根据配置幂等应用自启动 / 开始菜单快捷方式
	applySystemSettings(cfg, log)

	log.Info("boardsorter 服务已完全启动")

	// 启动 IPC HTTP 服务
	if noIPC {
		log.Info("已通过 --no-ipc 跳过 IPC 服务启动")
		return
	}

	triggerManualScan = func() (int, error) {
		n0, _ := metadata.GetStats()
		class.ScanSubjectFolders(cfg.SubjectFolders)
		n1, _ := metadata.GetStats()
		return n1 - n0, nil
	}
	onStop := func() {
		log.Info("收到停止信号...")
		if appMonitor != nil {
			appMonitor.Stop()
		}
		if appDelDeleter != nil {
			appDelDeleter.Stop()
		}
		log.Info("程序已退出")
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}
	port, err := StartIPCServer(cfg, termDB, metadata, class, log, mon, delDeleter, onStop)
	if err != nil {
		log.Error("启动IPC服务失败: %v", err)
	} else {
		log.Info("IPC服务已启动: http://%s:%d", cfg.IPCBindHost, port)
	}
}

// applySystemSettings 幂等地根据 cfg.AutoStart / cfg.StartMenuLink 同步系统状态
func applySystemSettings(cfg *Config, log *Logger) {
	execPath, err := os.Executable()
	if err != nil {
		log.Warn("获取可执行文件路径失败，跳过系统设置: %v", err)
		return
	}

	// 1) 开机自启
	if cfg.AutoStart {
		curEnabled, _ := IsAutoStartEnabled()
		curPath, _ := GetAutoStartExePath()
		if !curEnabled || !samePath(curPath, execPath) {
			if err := SetAutoStart(true, execPath); err != nil {
				log.Warn("启用开机自启失败: %v", err)
			} else {
				log.Info("已启用开机自启: %s", execPath)
			}
		} else {
			log.Info("开机自启已是最新状态，无需更新")
		}
	} else {
		curEnabled, _ := IsAutoStartEnabled()
		if curEnabled {
			if err := SetAutoStart(false, execPath); err != nil {
				log.Warn("禁用开机自启失败: %v", err)
			} else {
				log.Info("已禁用开机自启")
			}
		}
	}

	// 2) 开始菜单快捷方式
	if cfg.StartMenuLink {
		if hasStartMenuShortcuts(appDisplayName) {
			log.Info("开始菜单快捷方式已存在，无需更新")
		} else {
			if err := CreateStartMenuShortcuts(execPath, appDisplayName); err != nil {
				log.Warn("创建开始菜单快捷方式失败: %v", err)
			} else {
				log.Info("已创建开始菜单快捷方式")
			}
		}
	} else {
		if hasStartMenuShortcuts(appDisplayName) {
			if err := RemoveStartMenuShortcuts(appDisplayName); err != nil {
				log.Warn("删除开始菜单快捷方式失败: %v", err)
			} else {
				log.Info("已删除开始菜单快捷方式")
			}
		}
	}
}

// hasStartMenuShortcuts 检查开始菜单 Programs / StartUp 下 .lnk 是否存在
func hasStartMenuShortcuts(appName string) bool {
	// 简单判断：尝试走和 CreateStartMenuShortcuts 相同的路径
	// 通过 system 包提供的辅助函数获取（系统实现里如果不存在则返回空）
	programsPath, err := getStartMenuProgramsPath()
	if err != nil {
		return false
	}
	mainLink := filepath.Join(programsPath, appName+".lnk")
	if _, err := os.Stat(mainLink); err == nil {
		return true
	}
	startupPath, err := getStartMenuStartUpPath()
	if err != nil {
		return false
	}
	startupLink := filepath.Join(startupPath, appName+".lnk")
	if _, err := os.Stat(startupLink); err == nil {
		return true
	}
	return false
}

// samePath 比较两个路径是否指向同一文件（忽略大小写与可能的 .. 等）
func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return strings.EqualFold(filepath.Clean(absA), filepath.Clean(absB))
}

// cleanupSystem 删除自启动和开始菜单快捷方式，然后退出
func cleanupSystem() {
	fmt.Println("正在清理系统集成项（开机自启 + 开始菜单快捷方式）...")
	if err := SetAutoStart(false, ""); err != nil {
		fmt.Fprintf(os.Stderr, "删除开机自启失败: %v\n", err)
	} else {
		fmt.Println("已删除开机自启项")
	}
	if err := RemoveStartMenuShortcuts(appDisplayName); err != nil {
		fmt.Fprintf(os.Stderr, "删除开始菜单快捷方式失败: %v\n", err)
	} else {
		fmt.Println("已删除开始菜单快捷方式")
	}
	fmt.Println("清理完成。")
}

func printHelp() {
	fmt.Println("boardsorter - 高中教学文件自动归档系统")
	fmt.Println("版本: 1.3")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  boardsorter                  启动程序（控制台模式）")
	fmt.Println("  boardsorter --console, -c    启动程序（控制台模式）")
	fmt.Println("  boardsorter --no-ipc         启动程序但不启动 IPC 服务（调试用）")
	fmt.Println("  boardsorter --cleanup-system 删除开机自启和开始菜单快捷方式后退出")
	fmt.Println("  boardsorter --help, -h       显示帮助信息")
	fmt.Println()
	fmt.Println("配置文件 config.ini 需与程序在同一目录")
}
