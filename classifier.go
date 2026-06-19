package main

import (
	"path/filepath"

)

// Classifier 文件分类器
type Classifier struct {
	cfg        *classifierConfig
	wordStore  *HotWordStore
	aiClient   *AIClient
	extractor  *Extractor
	archiver   *Archiver
	deleter    *DelayedDeleter
	logFn func(string, ...interface{})
}

type classifierConfig struct {
	Subjects           []string
	ArchiveRoot        string
	IrrelevantFolder   string
	UncertainFolder    string
	IrrelevantCategory string
	UncertainCategory  string
}

// NewClassifier 创建分类器
func NewClassifier(
	subjects []string,
	archiveRoot, irrelevantFolder, uncertainFolder string,
	wordStore *HotWordStore,
	aiClient *AIClient,
	extractor *Extractor,
	archiver *Archiver,
	deleter *DelayedDeleter,
	logFn func(string, ...interface{}),
) *Classifier {
	return &Classifier{
		cfg: &classifierConfig{
			Subjects:           subjects,
			ArchiveRoot:        archiveRoot,
			IrrelevantFolder:   irrelevantFolder,
			UncertainFolder:    uncertainFolder,
			IrrelevantCategory: "无关文件",
			UncertainCategory:  "无法确定",
		},
		wordStore: wordStore,
		aiClient:  aiClient,
		extractor: extractor,
		archiver:  archiver,
		deleter:   deleter,
		logFn:     logFn,
	}
}

// isSubject 判断是否为已知学科
func (c *Classifier) isSubject(category string) bool {
	for _, s := range c.cfg.Subjects {
		if s == category {
			return true
		}
	}
	return false
}

// ClassifyFile 对文件执行完整分类流程
func (c *Classifier) ClassifyFile(filePath string) {
	fileName := filepath.Base(filePath)

	// Step 1: 热词匹配
	if subject, word := c.wordStore.MatchHot(fileName); subject != "" {
		c.logFn("[INFO] [热词匹配] 关键词\"%s\"匹配科目\"%s\"", word, subject)
		c.archiveTo(filePath, subject)
		return
	}

	// Step 2: 冷词匹配
	if subject, word := c.wordStore.MatchCold(fileName); subject != "" {
		c.logFn("[INFO] [冷词匹配] 关键词\"%s\"匹配科目\"%s\"", word, subject)
		c.archiveTo(filePath, subject)
		return
	}

	// Step 3: AI轻量分析（仅文件名）
	c.lightAnalysis(filePath, fileName)
}

// lightAnalysis AI轻量分析（仅文件名）
func (c *Classifier) lightAnalysis(filePath, fileName string) {
	result, err := c.aiClient.Analyze(fileName, true)
	if err != nil {
		c.logFn("[ERROR] [轻量分析] %s 失败: %v", fileName, err)
		c.archiveTo(filePath, c.cfg.UncertainFolder)
		return
	}
	c.handleAIResult(filePath, fileName, result)
}

// handleAIResult 处理AI返回结果
func (c *Classifier) handleAIResult(filePath, fileName string, result *AIResult) {
	if result == nil {
		c.archiveTo(filePath, c.cfg.UncertainFolder)
		return
	}

	category := result.Category
	keywords := result.Keywords

	c.logFn("[INFO] [AI分析] \"%s\" -> 类别: %s, 关键词: %v", fileName, category, keywords)

	// 属于六个学科 -> 学习热词并归档
	if c.isSubject(category) {
		if len(keywords) > 0 {
			c.wordStore.AddHotWords(category, keywords)
			c.logFn("[INFO] [热词学习] 科目\"%s\"新增热词: %v", category, keywords)
		}
		c.archiveTo(filePath, category)
		return
	}

	// 无关文件 -> 直接归档，不提取热词
	if category == c.cfg.IrrelevantCategory {
		c.archiveTo(filePath, c.cfg.IrrelevantFolder)
		return
	}

	// 无法确定 -> 尝试深度分析
	if category == c.cfg.UncertainCategory {
		c.deepAnalysis(filePath, fileName)
		return
	}

	// 未知类别 -> 无法确定
	c.archiveTo(filePath, c.cfg.UncertainFolder)
}

// deepAnalysis AI深度分析（提取文件内容）
func (c *Classifier) deepAnalysis(filePath, fileName string) {
	if !c.extractor.CanExtract(filePath) {
		c.logFn("[INFO] [深度分析] 文件类型不支持提取内容: %s, 归入无法确定", fileName)
		c.archiveTo(filePath, c.cfg.UncertainFolder)
		return
	}

	content, err := c.extractor.Extract(filePath)
	if err != nil {
		c.logFn("[WARN] [深度分析] 提取内容失败: %s, 错误: %v", fileName, err)
		c.archiveTo(filePath, c.cfg.UncertainFolder)
		return
	}

	c.logFn("[INFO] [深度分析] 提取到内容(%d字符), 正在分析: %s", len([]rune(content)), fileName)

	result, err := c.aiClient.Analyze(content, false)
	if err != nil {
		c.logFn("[ERROR] [深度分析] AI调用失败: %v", err)
		c.archiveTo(filePath, c.cfg.UncertainFolder)
		return
	}

	if result == nil {
		c.archiveTo(filePath, c.cfg.UncertainFolder)
		return
	}

	c.logFn("[INFO] [深度分析结果] \"%s\" -> 类别: %s", fileName, result.Category)

	if c.isSubject(result.Category) {
		if len(result.Keywords) > 0 {
			c.wordStore.AddHotWords(result.Category, result.Keywords)
		}
		c.archiveTo(filePath, result.Category)
		return
	}

	if result.Category == c.cfg.IrrelevantCategory {
		c.archiveTo(filePath, c.cfg.IrrelevantFolder)
		return
	}

	c.archiveTo(filePath, c.cfg.UncertainFolder)
}

// archiveTo 执行归档操作
func (c *Classifier) archiveTo(filePath string, target string) {
	var destDir string

	if c.isSubject(target) {
		destDir = filepath.Join(c.cfg.ArchiveRoot, target)
	} else if target == c.cfg.IrrelevantFolder || target == c.cfg.IrrelevantCategory {
		destDir = c.cfg.IrrelevantFolder
	} else {
		destDir = c.cfg.UncertainFolder
	}

	_, err := c.archiver.Archive(filePath, destDir)
	if err != nil {
		c.logFn("[ERROR] [归档失败] %s, 错误: %v", filePath, err)
		return
	}

	// 加入延迟删除队列
	c.deleter.Add(filePath)
}