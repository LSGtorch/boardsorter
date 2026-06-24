package main

import (
	"fmt"
	"os"
	"path/filepath"
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
	appLog       *Logger
	appTermDB    *TermDB
	appMetadata  *FileMetadata
	appDelDeleter *DelayedDeleter
	appMonitor   *Monitor
	appTray      *TrayApp
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
		case "--autostart", "-a":
			enableAutoStart()
			return
		case "--no-autostart", "-na":
			disableAutoStart()
			return
		case "--console", "-c":
			runConsole(execDir)
			return
		}
	}

	// 默认带托盘运行
	runWithTray(execDir)
}

func runConsole(execDir string) {
	cfg, log := initSystem(execDir)
	if cfg == nil || log == nil {
		fmt.Println("按回车键退出...")
		fmt.Scanln()
		os.Exit(1)
	}
	startServer(cfg, log)
	select {} // 保持运行
}

func runWithTray(execDir string) {
	cfg, log := initSystem(execDir)
	if cfg == nil || log == nil {
		fmt.Println("按回车键退出...")
		fmt.Scanln()
		os.Exit(1)
	}

	log.Info("初始化系统托盘...")

	// TrayApp.Run() 会阻塞主 goroutine
	tray := NewTrayApp(
		func() { startServer(cfg, log) },
		func() {
			log.Info("正在关闭程序...")
			if appMonitor != nil {
				appMonitor.Stop()
			}
			if appDelDeleter != nil {
				appDelDeleter.Stop()
			}
			log.Info("程序已退出")
		},
	)

	log.Info("boardsorter 已在系统托盘运行，点击托盘图标查看日志")
	log.Info("运行模式: 托盘模式（右键菜单可查看日志或退出）")
	appTray = tray
	// 将日志回调注册到托盘，让托盘可以显示日志
	log.RegisterCallback(tray.addLog)
	tray.Run()
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

func startServer(cfg *Config, log *Logger) {
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

	log.Info("boardsorter 服务已完全启动")
}

// enableAutoStart 启用开机自启（Windows）
func enableAutoStart() {
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取程序路径失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("启用开机自启: %s\n", execPath)
	fmt.Println("请在Windows环境下运行以下命令或手动添加开机启动项:")
	fmt.Printf("  reg add \"HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run\" /v boardsorter /t REG_SZ /d \"%s\" /f\n", execPath)
	fmt.Println("或者将程序快捷方式放入: %APPDATA%\\Microsoft\\Windows\\Start Menu\\Programs\\Startup")
}

// disableAutoStart 禁用开机自启
func disableAutoStart() {
	fmt.Println("禁用开机自启:")
	fmt.Println("  reg delete \"HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run\" /v boardsorter /f")
}

func printHelp() {
	fmt.Println("boardsorter - 高中教学文件自动归档系统")
	fmt.Println("版本: 1.3")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  boardsorter                  启动程序（带系统托盘）")
	fmt.Println("  boardsorter --console        控制台模式（无托盘）")
	fmt.Println("  boardsorter --autostart      启用开机自启")
	fmt.Println("  boardsorter --no-autostart   禁用开机自启")
	fmt.Println("  boardsorter --help           显示帮助信息")
	fmt.Println()
	fmt.Println("配置文件 config.ini 需与程序在同一目录")
}
