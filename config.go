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
	AIEndpoint       string
	APIKey           string
	ModelName        string
	AIPrompt         string
	RetryWaitSec     int
	MaxRetries       int
	ReasoningEffort  string

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
	defaultRetryWaitSec     = 60
	defaultMaxRetries       = 1
	defaultHotDegradeDays   = 7
	defaultColdDeleteDays   = 30
	defaultSourceRetainHour = 1
	defaultScanInterval     = 5
	defaultReadableExts     = ".docx,.pptx,.pdf,.txt"
	defaultArchiveExts      = ".zip,.rar,.7z"
	defaultAIEndpoint       = "https://api.deepseek.com/v1/chat/completions"
	defaultModelName        = "deepseek-v4-flash"
	defaultReasoningEffort  = "low"
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
		SubjectFolders:   append([]string{}, defaultSubjects...),
		AIPrompt:         defaultPrompt,
		AIEndpoint:       defaultAIEndpoint,
		ModelName:        defaultModelName,
		ReasoningEffort:  defaultReasoningEffort,
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
		case "系统提示词":
			if value != "" {
				c.AIPrompt = value
			}
		case "推理等级":
			if value != "" {
				c.ReasoningEffort = value
			}
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

// defaultPrompt 默认系统提示词
const defaultPrompt = `你是中国高中教学文件归类专家。从文件名判断类别：【类别】数学,语文,英语,物理,化学,生物,无关文件,无法确定。【判断规则】1.文件名中模棱两可的词不作为判断依据，跳过继续找其他关键词。模棱两可词包括跨学科交叉词：糖类、蛋白质、油脂（化学和生物都有）、能量（物理和化学都有）、计算（数学和物理都有）等。2.只根据明确的学科关键词归类。【英语关键词】定语从句、状语从句、非谓语动词、倒装句、虚拟语气、主谓一致、被动语态、语法填空、七选五、改错、书面表达、完形填空、阅读理解、单词、句型、听力、作文模板。【各科关键词】物理：牛顿定律、加速度、受力分析、摩擦、动量、能量守恒、电磁感应、欧姆定律、电路、电场、磁场、热学、光学、声学、原子物理、物理实验。数学：三角函数、向量、导数、函数、数列、不等式、解析几何、立体几何、概率、统计、代数、方程。语文：文言文、古诗词、现代文阅读、作文、修辞、成语、文学常识。化学：元素周期表、化学反应、化学方程式、有机化学、无机化学、化学实验、化学反应与能量。生物：细胞分裂、基因、光合作用、遗传、生态系统、生物实验、有丝分裂、减数分裂。【注意事项】1.去掉年级（七年级/八年级/高一/高二/高三等）、通用词（课件/教案/试卷/练习/复习/期末/期中/月考）、文件后缀（.pptx/.docx/.pdf）后，没有明确学科关键词→"无法确定"。2.后缀永不进keywords。3."定语从句""状语从句"等英语语法术语→英语。4.糖类、蛋白质、油脂等化学/生物交叉词不作为归类依据。5.文件名或正文内容大部分为英文→英语。【输出严格JSON】{"category":"英语","keywords":["定语从句"]} 或 {"category":"无法确定","keywords":[]}`
