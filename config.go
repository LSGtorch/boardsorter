package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config 存储所有配置项
type Config struct {
	// 路径配置
	ArchiveRoot      string
	SubjectFolders   []string // 六个学科名称
	IrrelevantFolder string
	UncertainFolder  string
	LogFolder        string

	// AI配置
	AIEndpoint    string
	APIKey        string
	ModelName     string
	RetryWaitSec  int
	MaxRetries    int

	// 规则配置
	HotDegradeDays     int
	ColdDeleteDays     int
	SourceRetainHour   int
	ReadableExts       string
	ArchiveExts        string

	// 监控配置
	WatchFolder  string
	ScanInterval int

	// 派生字段
	ReadableExtList []string
	ArchiveExtList  []string
}

// 默认值
const (
	defaultRetryWaitSec    = 60
	defaultMaxRetries      = 1
	defaultHotDegradeDays  = 7
	defaultColdDeleteDays  = 30
	defaultSourceRetainHour = 1
	defaultScanInterval    = 5
	defaultReadableExts    = ".docx,.pptx,.pdf,.txt"
	defaultArchiveExts     = ".zip,.rar,.7z"
)

// 默认科目
var defaultSubjects = []string{"数学", "语文", "英语", "物理", "化学", "生物"}

// LoadConfig 从指定路径加载配置文件
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		RetryWaitSec:     defaultRetryWaitSec,
		MaxRetries:       defaultMaxRetries,
		HotDegradeDays:   defaultHotDegradeDays,
		ColdDeleteDays:   defaultColdDeleteDays,
		SourceRetainHour: defaultSourceRetainHour,
		ScanInterval:     defaultScanInterval,
		ReadableExts:     defaultReadableExts,
		ArchiveExts:      defaultArchiveExts,
		SubjectFolders:   append([]string{}, defaultSubjects...), // 复制默认值
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("无法打开配置文件 %s: %w", path, err)
	}
	defer f.Close()

	currentSection := ""
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		// 解析section
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = line[1 : len(line)-1]
			continue
		}

		// 解析键值对
		eqIdx := strings.Index(line, "=")
		if eqIdx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eqIdx])
		value := strings.TrimSpace(line[eqIdx+1:])

		if err := cfg.setField(currentSection, key, value); err != nil {
			return nil, fmt.Errorf("配置文件 %s 解析错误: %w", path, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取配置文件出错: %w", err)
	}

	// 派生字段
	cfg.ReadableExtList = parseExtList(cfg.ReadableExts)
	cfg.ArchiveExtList = parseExtList(cfg.ArchiveExts)

	// 验证必要配置
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// setField 根据 section 和 key 赋值
func (c *Config) setField(section, key, value string) error {
	switch section {
	case "路径配置":
		switch key {
		case "归档根目录":
			c.ArchiveRoot = value
		case "科目文件夹列表":
			parts := strings.Split(value, ",")
			c.SubjectFolders = nil
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					c.SubjectFolders = append(c.SubjectFolders, p)
				}
			}
		case "无关文件夹":
			c.IrrelevantFolder = value
		case "无法确定类别文件夹", "无法确定文件夹":
			c.UncertainFolder = value
		case "日志文件夹":
			c.LogFolder = value
		}

	case "AI配置":
		switch key {
		case "AI接口地址":
			c.AIEndpoint = value
		case "API密钥":
			c.APIKey = value
		case "模型名称":
			c.ModelName = value
		case "失败重试等待秒数":
			if n, ok := parseInt(value); ok {
				c.RetryWaitSec = n
			}
		case "最大重试次数":
			if n, ok := parseInt(value); ok {
				c.MaxRetries = n
			}
		}

	case "规则配置":
		switch key {
		case "热词未使用降级天数":
			if n, ok := parseInt(value); ok {
				c.HotDegradeDays = n
			}
		case "冷词未使用删除天数":
			if n, ok := parseInt(value); ok {
				c.ColdDeleteDays = n
			}
		case "下载源文件保留小时数":
			if n, ok := parseInt(value); ok {
				c.SourceRetainHour = n
			}
		case "可读文档扩展名":
			c.ReadableExts = value
		case "压缩包扩展名":
			c.ArchiveExts = value
		}

	case "监控配置":
		switch key {
		case "要监控的下载文件夹":
			c.WatchFolder = value
		case "扫描间隔秒数":
			if n, ok := parseInt(value); ok {
				c.ScanInterval = n
			}
		}
	}

	return nil
}

func parseExtList(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			if !strings.HasPrefix(p, ".") {
				p = "." + p
			}
			result = append(result, p)
		}
	}
	return result
}

func parseInt(s string) (int, bool) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	if err != nil {
		return 0, false
	}
	return result, true
}

// Validate 验证配置是否完整
func (c *Config) Validate() error {
	if c.WatchFolder == "" {
		return fmt.Errorf("配置错误：要监控的下载文件夹不能为空")
	}
	if c.ArchiveRoot == "" {
		return fmt.Errorf("配置错误：归档根目录不能为空")
	}
	if len(c.SubjectFolders) == 0 {
		c.SubjectFolders = append([]string{}, defaultSubjects...)
	}
	if c.IrrelevantFolder == "" {
		c.IrrelevantFolder = filepath.Join(c.ArchiveRoot, "其他无关文件")
	}
	if c.UncertainFolder == "" {
		c.UncertainFolder = filepath.Join(c.ArchiveRoot, "无法确定类别")
	}
	if c.LogFolder == "" {
		c.LogFolder = filepath.Join(c.ArchiveRoot, "程序日志")
	}
	return nil
}

// EnsureDirectories 确保所有必要的目录存在
func (c *Config) EnsureDirectories() error {
	dirs := []string{c.ArchiveRoot, c.IrrelevantFolder, c.UncertainFolder, c.LogFolder}
	for _, subject := range c.SubjectFolders {
		dirs = append(dirs, filepath.Join(c.ArchiveRoot, subject))
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("创建目录失败 %s: %w", d, err)
		}
	}
	return nil
}

// GetSubjectPath 获取学科文件夹路径
func (c *Config) GetSubjectPath(subject string) string {
	return filepath.Join(c.ArchiveRoot, subject)
}

// SubjectList 返回学科名称列表（带引号方便打印）
func (c *Config) SubjectList() string {
	parts := make([]string, len(c.SubjectFolders))
	for i, s := range c.SubjectFolders {
		parts[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(parts, ", ")
}
