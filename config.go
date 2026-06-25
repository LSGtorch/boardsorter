package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	SourceRetainHour   int
	TermMaxIdleDays    int
	ReadableExts       string
	ArchiveExts        string

	// 监控配置
	WatchFolder  string
	ScanInterval int

	// IPC配置
	IPCPort         int    // 0=随机，>0=固定
	IPCBindHost     string // 默认 127.0.0.1

	// 启动项配置
	AutoStart        bool // 开机自启动
	StartMenuLink    bool // 开始菜单快捷方式

	// UI配置
	DarkMode bool // 深色模式

	// ClassIsland 通知配置
	ClassIslandNotifyEnabled  bool   // 是否启用分类通知
	ClassIslandNotifyURL      string // ClassIsland 导航 URI（通过 IPC 调用 IPublicUriNavigationService）
	ClassIslandNotifyTemplate string // 通知模板

	// 派生字段
	ReadableExtList []string
	ArchiveExtList  []string
}

// 默认值
const (
	defaultRetryWaitSec     = 60
	defaultMaxRetries       = 1
	defaultSourceRetainHour = 1
	defaultScanInterval     = 5
	defaultReadableExts     = ".docx,.pptx,.pdf,.txt"
	defaultArchiveExts      = ".zip,.rar,.7z"
	defaultAIEndpoint       = "https://api.deepseek.com/v1/chat/completions"
	defaultModelName        = "deepseek-v4-flash"
	defaultReasoningEffort  = "low"
	defaultTermMaxIdleDays  = 30
	defaultIPCBindHost      = "127.0.0.1"
)

// 默认科目
var defaultSubjects = []string{"数学", "语文", "英语", "物理", "化学", "生物"}

// LoadConfig 从指定路径加载配置文件
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		RetryWaitSec:     defaultRetryWaitSec,
		MaxRetries:       defaultMaxRetries,
		SourceRetainHour: defaultSourceRetainHour,
		ScanInterval:     defaultScanInterval,
		ReadableExts:     defaultReadableExts,
		ArchiveExts:      defaultArchiveExts,
		TermMaxIdleDays:  defaultTermMaxIdleDays,
		SubjectFolders:   append([]string{}, defaultSubjects...),
		AIPrompt:         defaultPrompt,
		AIEndpoint:       defaultAIEndpoint,
		ModelName:        defaultModelName,
		ReasoningEffort:  defaultReasoningEffort,
		IPCBindHost:      defaultIPCBindHost,
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
		case "下载源文件保留小时数":
			if n, ok := parseInt(value); ok {
				c.SourceRetainHour = n
			}
		case "词条最大空闲天数":
			if n, ok := parseInt(value); ok {
				c.TermMaxIdleDays = n
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

	case "IPC配置":
		switch key {
		case "IPC端口":
			if n, ok := parseInt(value); ok {
				c.IPCPort = n
			}
		case "IPC绑定地址":
			if value != "" {
				c.IPCBindHost = value
			}
		}

	case "启动配置":
		switch key {
		case "开机自启动":
			if b, ok := parseBool(value); ok {
				c.AutoStart = b
			}
		case "开始菜单快捷方式":
			if b, ok := parseBool(value); ok {
				c.StartMenuLink = b
			}
		}

	case "UI配置":
		switch key {
		case "深色模式":
			if b, ok := parseBool(value); ok {
				c.DarkMode = b
			}
		}

	case "ClassIsland通知":
		switch key {
		case "启用通知":
			if b, ok := parseBool(value); ok {
				c.ClassIslandNotifyEnabled = b
			}
		case "API地址":
			c.ClassIslandNotifyURL = value
		case "通知模板":
			c.ClassIslandNotifyTemplate = value
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

func parseBool(s string) (bool, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "true", "yes", "1", "on", "是", "启用", "开":
		return true, true
	case "false", "no", "0", "off", "否", "禁用", "关":
		return false, true
	}
	return false, false
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
	if c.IPCPort == 0 {
		// 0 表示随机端口，合法
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

// SaveConfig 把 Config 重新序列化成 INI 格式写回 path。
// 保留所有注释、分节顺序和原有顺序；如果 cfg 中某字段值为空，则跳过该字段保留原行不变。
func SaveConfig(path string, cfg *Config) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("无法打开配置文件 %s: %w", path, err)
	}

	var lines []string
	currentSection := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			currentSection = trimmed[1 : len(trimmed)-1]
			lines = append(lines, line)
			continue
		}

		if trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			lines = append(lines, line)
			continue
		}

		eqIdx := strings.Index(trimmed, "=")
		if eqIdx < 0 {
			lines = append(lines, line)
			continue
		}
		key := strings.TrimSpace(trimmed[:eqIdx])

		newValue, hasField := configValueString(currentSection, key, cfg)
		if !hasField || newValue == "" {
			// 不识别或值为空：保留原行
			lines = append(lines, line)
			continue
		}

		// 保留前导缩进
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		lines = append(lines, indent+key+" = "+newValue)
	}
	if err := scanner.Err(); err != nil {
		f.Close()
		return fmt.Errorf("读取配置文件出错: %w", err)
	}
	f.Close()

	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("无法写入配置文件 %s: %w", path, err)
	}
	defer out.Close()

	writer := bufio.NewWriter(out)
	for _, l := range lines {
		if _, err := writer.WriteString(l + "\n"); err != nil {
			return err
		}
	}
	return writer.Flush()
}

// configValueString 根据 section/key 返回 cfg 中对应字段的字符串值。
// 返回 (值, 是否识别)。识别但值为空时返回 ("", true)，由调用方决定如何处理。
func configValueString(section, key string, cfg *Config) (string, bool) {
	switch section {
	case "路径配置":
		switch key {
		case "归档根目录":
			return cfg.ArchiveRoot, true
		case "科目文件夹列表":
			return strings.Join(cfg.SubjectFolders, ", "), true
		case "无关文件夹":
			return cfg.IrrelevantFolder, true
		case "无法确定类别文件夹", "无法确定文件夹":
			return cfg.UncertainFolder, true
		case "日志文件夹":
			return cfg.LogFolder, true
		}
	case "AI配置":
		switch key {
		case "AI接口地址":
			return cfg.AIEndpoint, true
		case "API密钥":
			return cfg.APIKey, true
		case "模型名称":
			return cfg.ModelName, true
		case "系统提示词":
			return cfg.AIPrompt, true
		case "推理等级":
			return cfg.ReasoningEffort, true
		case "失败重试等待秒数":
			return strconv.Itoa(cfg.RetryWaitSec), true
		case "最大重试次数":
			return strconv.Itoa(cfg.MaxRetries), true
		}
	case "规则配置":
		switch key {
		case "下载源文件保留小时数":
			return strconv.Itoa(cfg.SourceRetainHour), true
		case "词条最大空闲天数":
			return strconv.Itoa(cfg.TermMaxIdleDays), true
		case "可读文档扩展名":
			return cfg.ReadableExts, true
		case "压缩包扩展名":
			return cfg.ArchiveExts, true
		}
	case "监控配置":
		switch key {
		case "要监控的下载文件夹":
			return cfg.WatchFolder, true
		case "扫描间隔秒数":
			return strconv.Itoa(cfg.ScanInterval), true
		}
	case "IPC配置":
		switch key {
		case "IPC端口":
			return strconv.Itoa(cfg.IPCPort), true
		case "IPC绑定地址":
			return cfg.IPCBindHost, true
		}
	case "启动配置":
		switch key {
		case "开机自启动":
			return strconv.FormatBool(cfg.AutoStart), true
		case "开始菜单快捷方式":
			return strconv.FormatBool(cfg.StartMenuLink), true
		}
	case "UI配置":
		switch key {
		case "深色模式":
			return strconv.FormatBool(cfg.DarkMode), true
		}
	case "ClassIsland通知":
		switch key {
		case "启用通知":
			return strconv.FormatBool(cfg.ClassIslandNotifyEnabled), true
		case "API地址":
			return cfg.ClassIslandNotifyURL, true
		case "通知模板":
			return cfg.ClassIslandNotifyTemplate, true
		}
	}
	return "", false
}

// defaultPrompt 默认系统提示词
const defaultPrompt = `你是中国高中教学文件归类专家。从文件名判断类别：【类别】数学,语文,英语,物理,化学,生物,无关文件,无法确定。【判断规则】1.文件名中模棱两可的词不作为判断依据，跳过继续找其他关键词。模棱两可词包括跨学科交叉词：糖类、蛋白质、油脂（化学和生物都有）、能量（物理和化学都有）、计算（数学和物理都有）等。2.只根据明确的学科关键词归类。【英语关键词】定语从句、状语从句、非谓语动词、倒装句、虚拟语气、主谓一致、被动语态、语法填空、七选五、改错、书面表达、完形填空、阅读理解、单词、句型、听力、作文模板。【各科关键词】物理：牛顿定律、加速度、受力分析、摩擦、动量、能量守恒、电磁感应、欧姆定律、电路、电场、磁场、热学、光学、声学、原子物理、物理实验。数学：三角函数、向量、导数、函数、数列、不等式、解析几何、立体几何、概率、统计、代数、方程。语文：文言文、古诗词、现代文阅读、作文、修辞、成语、文学常识。化学：元素周期表、化学反应、化学方程式、有机化学、无机化学、化学实验、化学反应与能量。生物：细胞分裂、基因、光合作用、遗传、生态系统、生物实验、有丝分裂、减数分裂。【注意事项】1.去掉年级（七年级/八年级/高一/高二/高三等）、通用词（课件/教案/试卷/练习/复习/期末/期中/月考）、文件后缀（.pptx/.docx/.pdf）后，没有明确学科关键词→"无法确定"。2.后缀永不进keywords。3."定语从句""状语从句"等英语语法术语→英语。4.糖类、蛋白质、油脂等化学/生物交叉词不作为归类依据。5.文件名或正文内容大部分为英文→英语。【输出严格JSON】{"category":"英语","keywords":["定语从句"]} 或 {"category":"无法确定","keywords":[]}`

// configFieldSpec 描述一个内置配置项的 (section, key, defaultValue)。
// 顺序即为写入文件时的顺序。
type configFieldSpec struct {
	Section       string
	Key           string
	DefaultValue  string
	NewInVersion  string // 若非空，表示"该字段是此版本新增"，追加时打注释
}

// allConfigFields 列出 boardsorter 当前识别的所有配置项。
// 任何新加字段都应在此登记，确保从老版本升级时能自动追加。
var allConfigFields = []configFieldSpec{
	// [路径配置]
	{Section: "路径配置", Key: "归档根目录"},
	{Section: "路径配置", Key: "科目文件夹列表"},
	{Section: "路径配置", Key: "无关文件夹"},
	{Section: "路径配置", Key: "无法确定类别文件夹"},
	{Section: "路径配置", Key: "日志文件夹"},

	// [AI配置]
	{Section: "AI配置", Key: "AI接口地址"},
	{Section: "AI配置", Key: "API密钥"},
	{Section: "AI配置", Key: "模型名称"},
	{Section: "AI配置", Key: "系统提示词"},
	{Section: "AI配置", Key: "推理等级"},
	{Section: "AI配置", Key: "失败重试等待秒数"},
	{Section: "AI配置", Key: "最大重试次数"},

	// [规则配置]
	{Section: "规则配置", Key: "下载源文件保留小时数"},
	{Section: "规则配置", Key: "词条最大空闲天数"},
	{Section: "规则配置", Key: "可读文档扩展名"},
	{Section: "规则配置", Key: "压缩包扩展名"},

	// [监控配置]
	{Section: "监控配置", Key: "要监控的下载文件夹"},
	{Section: "监控配置", Key: "扫描间隔秒数"},

	// [IPC配置] - v1.3 新增
	{Section: "IPC配置", Key: "IPC端口", NewInVersion: "v1.3"},
	{Section: "IPC配置", Key: "IPC绑定地址", NewInVersion: "v1.3"},

	// [启动配置] - v1.3 新增
	{Section: "启动配置", Key: "开机自启动", NewInVersion: "v1.3"},
	{Section: "启动配置", Key: "开始菜单快捷方式", NewInVersion: "v1.3"},

	// [UI配置] - v1.3 新增
	{Section: "UI配置", Key: "深色模式", NewInVersion: "v1.3"},

	// [ClassIsland通知] - v1.3 新增（通过 dotnetCampus.Ipc 命名管道通信）
	{Section: "ClassIsland通知", Key: "启用通知", NewInVersion: "v1.3"},
	{Section: "ClassIsland通知", Key: "API地址", NewInVersion: "v1.3"},
	{Section: "ClassIsland通知", Key: "通知模板", NewInVersion: "v1.3"},
}

// defaultValueForField 返回某个字段的默认值。
// 优先使用 config.go 中已定义的常量。
func defaultValueForField(section, key string) string {
	switch section {
	case "AI配置":
		switch key {
		case "AI接口地址":
			return defaultAIEndpoint
		case "API密钥":
			return ""
		case "模型名称":
			return defaultModelName
		case "系统提示词":
			return defaultPrompt
		case "推理等级":
			return defaultReasoningEffort
		case "失败重试等待秒数":
			return strconv.Itoa(defaultRetryWaitSec)
		case "最大重试次数":
			return strconv.Itoa(defaultMaxRetries)
		}
	case "规则配置":
		switch key {
		case "下载源文件保留小时数":
			return strconv.Itoa(defaultSourceRetainHour)
		case "词条最大空闲天数":
			return strconv.Itoa(defaultTermMaxIdleDays)
		case "可读文档扩展名":
			return defaultReadableExts
		case "压缩包扩展名":
			return defaultArchiveExts
		}
	case "监控配置":
		switch key {
		case "要监控的下载文件夹":
			return ""
		case "扫描间隔秒数":
			return strconv.Itoa(defaultScanInterval)
		}
	case "IPC配置":
		switch key {
		case "IPC端口":
			return "0"
		case "IPC绑定地址":
			return defaultIPCBindHost
		}
	case "启动配置":
		switch key {
		case "开机自启动":
			return "false"
		case "开始菜单快捷方式":
			return "false"
		}
	case "路径配置":
		switch key {
		case "科目文件夹列表":
			return strings.Join(defaultSubjects, ", ")
		}
	case "ClassIsland通知":
		switch key {
		case "启用通知":
			return "false"
		case "API地址":
			return "classisland://app/"
		case "通知模板":
			return "{filename} → {subject}"
		}
	}
	return ""
}

// parseExistingConfig 读取 ini 文件，统计所有已存在的 (section, key) 集合。
// 同时返回文件原有内容（用于在末尾追加）。
func parseExistingConfig(path string) (map[string]map[string]bool, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	existing := make(map[string]map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	// 允许长行（如默认 prompt）
	scanner.Buffer(make([]byte, 1024*1024), 8*1024*1024)

	currentSection := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = line[1 : len(line)-1]
			continue
		}
		eqIdx := strings.Index(line, "=")
		if eqIdx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eqIdx])
		if existing[currentSection] == nil {
			existing[currentSection] = make(map[string]bool)
		}
		existing[currentSection][key] = true
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	return existing, data, nil
}

// appendMissingDefaults 检查 path 指向的 ini 文件，追加所有缺失的 section/key。
// 不会修改已有字段的值（包括值为空的字段）。
// 多次调用是幂等的：第二次调用不会有任何 missing 项，因此不会重复追加。
func appendMissingDefaults(path string) error {
	existing, data, err := parseExistingConfig(path)
	if err != nil {
		return fmt.Errorf("读取配置 %s 失败: %w", path, err)
	}

	// 找出缺失的字段，按 (section, 在 allConfigFields 中的顺序) 分组
	type missingKey struct {
		Key        string
		DefaultVal string
		NewInVer   string
	}
	missingBySection := make(map[string][]missingKey)
	var sectionOrder []string
	seenSection := make(map[string]bool)

	for _, f := range allConfigFields {
		// 兼容老配置中可能使用的旧 key 名
		keysToCheck := []string{f.Key}
		if f.Section == "路径配置" && f.Key == "无法确定类别文件夹" {
			keysToCheck = append(keysToCheck, "无法确定文件夹")
		}
		matched := false
		for _, k := range keysToCheck {
			if existing[f.Section][k] {
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if !seenSection[f.Section] {
			seenSection[f.Section] = true
			sectionOrder = append(sectionOrder, f.Section)
		}
		missingBySection[f.Section] = append(missingBySection[f.Section], missingKey{
			Key:        f.Key,
			DefaultVal: defaultValueForField(f.Section, f.Key),
			NewInVer:   f.NewInVersion,
		})
	}

	if len(missingBySection) == 0 {
		return nil
	}

	// 追加到文件末尾
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("打开配置 %s 以追加失败: %w", path, err)
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	// 原文件末尾若不是以 \n 结尾，先补一个换行
	if len(data) > 0 && data[len(data)-1] != '\n' {
		if _, err := writer.WriteString("\n"); err != nil {
			return err
		}
	}
	// 空行 + 注释
	if _, err := writer.WriteString("\n; =============================================\n"); err != nil {
		return err
	}
	if _, err := writer.WriteString("; 以下为 boardsorter v" + appVersion + " 自动追加的配置项\n"); err != nil {
		return err
	}
	if _, err := writer.WriteString("; 请勿删除此段；老配置中没有的字段已补齐默认值\n"); err != nil {
		return err
	}
	if _, err := writer.WriteString("; =============================================\n"); err != nil {
		return err
	}

	for _, sec := range sectionOrder {
		if _, err := writer.WriteString("\n[" + sec + "]\n"); err != nil {
			return err
		}
		for _, mk := range missingBySection[sec] {
			if mk.NewInVer != "" {
				if _, err := writer.WriteString("; " + mk.NewInVer + " 新增\n"); err != nil {
					return err
				}
			}
			if _, err := writer.WriteString(mk.Key + " = " + mk.DefaultVal + "\n"); err != nil {
				return err
			}
		}
	}

	return writer.Flush()
}
