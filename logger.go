package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// LogCallback 日志回调，用于托盘界面显示
type LogCallback func(level string, msg string)

// Logger 双路日志系统
type Logger struct {
	mu          sync.Mutex
	logDir      string
	consoleLevel Level
	fileLevel   Level
	currentDate string
	fileHandle  *os.File
	maxDays     int
	callbacks   []LogCallback
}

// NewLogger 创建日志系统
func NewLogger(logDir string) (*Logger, error) {
	l := &Logger{
		logDir:       logDir,
		consoleLevel: INFO,
		fileLevel:    DEBUG,
		maxDays:      30,
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}
	if err := l.rotateFile(); err != nil {
		return nil, fmt.Errorf("初始化日志文件失败: %w", err)
	}
	// 启动后台清理旧日志
	go l.cleanupRoutine()
	return l, nil
}

// RegisterCallback 注册日志回调
func (l *Logger) RegisterCallback(cb LogCallback) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.callbacks = append(l.callbacks, cb)
}

func (l *Logger) rotateFile() error {
	date := time.Now().Format("2006-01-02")
	if l.fileHandle != nil && l.currentDate == date {
		return nil
	}
	if l.fileHandle != nil {
		l.fileHandle.Close()
	}
	l.currentDate = date
	logPath := filepath.Join(l.logDir, fmt.Sprintf("boardsorter-%s.log", date))
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	l.fileHandle = f
	return nil
}

func (l *Logger) cleanupRoutine() {
	for {
		time.Sleep(24 * time.Hour)
		l.mu.Lock()
		entries, err := os.ReadDir(l.logDir)
		l.mu.Unlock()
		if err != nil {
			continue
		}
		cutoff := time.Now().AddDate(0, 0, -l.maxDays)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				os.Remove(filepath.Join(l.logDir, entry.Name()))
			}
		}
	}
}

func (l *Logger) log(level Level, format string, args ...interface{}) {
	now := time.Now()
	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("[%s] [%s] %s", now.Format("2006-01-02 15:04:05"), level.String(), msg)

	l.mu.Lock()
	defer l.mu.Unlock()

	// 输出到控制台
	if level >= l.consoleLevel {
		var color string
		switch level {
		case WARN:
			color = "\033[33m"
		case ERROR:
			color = "\033[31m"
		default:
			color = "\033[0m"
		}
		if level >= WARN {
			fmt.Fprintln(os.Stderr, color+line+"\033[0m")
		} else {
			fmt.Fprintln(os.Stdout, line)
		}
	}

	// 输出到文件
	if level >= l.fileLevel {
		l.rotateFile()
		if l.fileHandle != nil {
			fmt.Fprintln(l.fileHandle, line)
		}
	}

	// 回调通知
	for _, cb := range l.callbacks {
		cb(level.String(), msg)
	}
}

// LogWriter 实现 io.Writer 接口，用于将标准库日志重定向
func (l *Logger) LogWriter(level Level) io.Writer {
	return &logWriter{l: l, level: level}
}

type logWriter struct {
	l     *Logger
	level Level
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.l.log(w.level, string(p))
	return len(p), nil
}

// Info 记录INFO级别日志
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn 记录WARN级别日志
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error 记录ERROR级别日志
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Debug 记录DEBUG级别日志
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Raw 输出原始消息（不添加级别前缀），用于已经包含级别标记的内部模块调用
func (l *Logger) Raw(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	now := time.Now()
	line := fmt.Sprintf("[%s] %s", now.Format("2006-01-02 15:04:05"), msg)

	l.mu.Lock()
	defer l.mu.Unlock()

	// 解析消息中的级别用于控制台颜色
	isWarn := strings.Contains(msg, "[WARN]")
	isError := strings.Contains(msg, "[ERROR]")
	if isWarn || isError {
		var color string
		if isError {
			color = "\033[31m"
		} else {
			color = "\033[33m"
		}
		fmt.Fprintln(os.Stderr, color+line+"\033[0m")
	} else {
		fmt.Fprintln(os.Stdout, line)
	}

	// 输出到文件
	l.rotateFile()
	if l.fileHandle != nil {
		fmt.Fprintln(l.fileHandle, line)
	}

	// 回调通知 - 解析级别
	level := "INFO"
	if isWarn {
		level = "WARN"
	} else if isError {
		level = "ERROR"
	}
	for _, cb := range l.callbacks {
		cb(level, msg)
	}
}

// Close 关闭日志系统
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.fileHandle != nil {
		l.fileHandle.Close()
		l.fileHandle = nil
	}
}