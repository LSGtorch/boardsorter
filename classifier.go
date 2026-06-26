package main

import (
	"path/filepath"
	"strings"
	"time"
)

// Classifier 文件分类器
type Classifier struct {
	cfg       *classifierConfig
	termDB    *TermDB
	metadata  *FileMetadata
	aiClient  *AIClient
	extractor *Extractor
	archiver  *Archiver
	deleter   *DelayedDeleter
	logFn     func(string, ...interface{})
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
	termDB *TermDB,
	metadata *FileMetadata,
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
		termDB:    termDB,
		metadata:  metadata,
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
// 流程：BM25词条匹配 → 命中则归档 → 否则AI轻量分析
func (c *Classifier) ClassifyFile(filePath string) {
	fileName := filepath.Base(filePath)

	// 检查是否已有UUID（处理RENAME后的更新）
	if existing := c.metadata.FindByPath(filePath); existing != nil {
		c.metadata.TouchLastSeen(existing.UUID)
		c.logFn("[DEBUG] 文件已存在UUID=%s: %s", existing.UUID, fileName)
		return
	}

	// Step 1: BM25 词条匹配
	result := c.termDB.MatchBM25(fileName)
	if result.HasMatch && c.isSubject(result.Subject) {
		c.logFn("[INFO] [BM25匹配] 科目=%s, 得分=%.3f, 命中关键词=%v",
			result.Subject, result.Score, result.Keywords)
		// 为命中词条增加词频
		newUUID := NewUUID()
		for _, kw := range result.Keywords {
			c.termDB.AddTerm(result.Subject, kw, newUUID)
		}
		c.archiveAndTrack(filePath, result.Subject, result.Keywords, SourceAuto)
		return
	}

	// Step 2: AI轻量分析（仅文件名）
	c.lightAnalysis(filePath, fileName)
}

// lightAnalysis AI轻量分析
func (c *Classifier) lightAnalysis(filePath, fileName string) {
	result, err := c.aiClient.Analyze(fileName, true)
	if err != nil {
		c.logFn("[ERROR] [轻量分析] %s 失败: %v", fileName, err)
		c.archiveAndTrack(filePath, c.cfg.UncertainCategory, nil, SourceAuto)
		return
	}
	c.handleAIResult(filePath, fileName, result)
}

// handleAIResult 处理AI返回结果
func (c *Classifier) handleAIResult(filePath, fileName string, result *AIResult) {
	if result == nil {
		c.archiveAndTrack(filePath, c.cfg.UncertainCategory, nil, SourceAuto)
		return
	}

	category := result.Category
	keywords := result.Keywords

	c.logFn("[INFO] [AI分析] \"%s\" -> 类别: %s, 关键词: %v", fileName, category, keywords)

	// 属于六大学科 -> 学习词条并归档
	if c.isSubject(category) {
		newUUID := NewUUID()
		c.termDB.AddTerms(category, keywords, newUUID)
		if len(keywords) > 0 {
			c.logFn("[INFO] [词条学习] 科目\"%s\"新增: %v", category, keywords)
		}
		c.archiveAndTrack(filePath, category, keywords, SourceAuto)
		return
	}

	// 无关文件
	if category == c.cfg.IrrelevantCategory {
		c.archiveAndTrack(filePath, c.cfg.IrrelevantCategory, nil, SourceAuto)
		return
	}

	// 无法确定 -> 尝试深度分析
	if category == c.cfg.UncertainCategory {
		c.deepAnalysis(filePath, fileName)
		return
	}

	// 未知类别
	c.archiveAndTrack(filePath, c.cfg.UncertainCategory, nil, SourceAuto)
}

// deepAnalysis AI深度分析
func (c *Classifier) deepAnalysis(filePath, fileName string) {
	if !c.extractor.CanExtract(filePath) {
		c.logFn("[INFO] [深度分析] 文件类型不支持提取内容: %s, 归入无法确定", fileName)
		c.archiveAndTrack(filePath, c.cfg.UncertainCategory, nil, SourceAuto)
		return
	}

	content, err := c.extractor.Extract(filePath)
	if err != nil {
		c.logFn("[WARN] [深度分析] 提取内容失败: %s, 错误: %v", fileName, err)
		c.archiveAndTrack(filePath, c.cfg.UncertainCategory, nil, SourceAuto)
		return
	}

	c.logFn("[INFO] [深度分析] 提取到内容(%d字符), 正在分析: %s", len([]rune(content)), fileName)

	result, err := c.aiClient.Analyze(content, false)
	if err != nil {
		c.logFn("[ERROR] [深度分析] AI调用失败: %v", err)
		c.archiveAndTrack(filePath, c.cfg.UncertainCategory, nil, SourceAuto)
		return
	}

	if result == nil {
		c.archiveAndTrack(filePath, c.cfg.UncertainCategory, nil, SourceAuto)
		return
	}

	c.logFn("[INFO] [深度分析结果] \"%s\" -> 类别: %s", fileName, result.Category)

	if c.isSubject(result.Category) {
		newUUID := NewUUID()
		c.termDB.AddTerms(result.Category, result.Keywords, newUUID)
		c.archiveAndTrack(filePath, result.Category, result.Keywords, SourceAuto)
		return
	}

	if result.Category == c.cfg.IrrelevantCategory {
		c.archiveAndTrack(filePath, c.cfg.IrrelevantCategory, nil, SourceAuto)
		return
	}

	c.archiveAndTrack(filePath, c.cfg.UncertainCategory, nil, SourceAuto)
}

// archiveAndTrack 执行归档并记录元数据
func (c *Classifier) archiveAndTrack(filePath, target string, keywords []string, source FileSource) {
	var destDir string

	if c.isSubject(target) {
		destDir = filepath.Join(c.cfg.ArchiveRoot, target)
	} else if target == c.cfg.IrrelevantCategory {
		destDir = c.cfg.IrrelevantFolder
	} else {
		destDir = c.cfg.UncertainFolder
	}

	destPath, err := c.archiver.Archive(filePath, destDir)
	if err != nil {
		c.logFn("[ERROR] [归档失败] %s, 错误: %v", filePath, err)
		return
	}

	// 创建元数据
	newUUID := NewUUID()
	entry := &FileEntry{
		UUID:         newUUID,
		OriginalName: filepath.Base(filePath),
		CurrentPath:  destPath,
		Subject:      target,
		Keywords:     keywords,
		CreatedAt:    time.Now(),
		LastSeen:     time.Now(),
		Source:       source,
	}
	c.metadata.Add(entry)
	c.logFn("[INFO] [元数据记录] UUID=%s, 路径=%s", newUUID, destPath)

	// 推送通知到队列（GUI 轮询后通过 Windows Toast 显示）
	PushNotification(filepath.Base(filePath), target)

	// 加入延迟删除队列
	c.deleter.Add(filePath)
}

// scanAndRegisterManualFiles 扫描用户手动放入的文件
// 该方法被 Monitor 周期调用
// 如果文件在metadata中已存在则更新last_seen
// 如果不存在则视为用户手动分类：分配UUID、记录到词条库
func (c *Classifier) ScanSubjectFolders(subjects []string) {
	for _, subject := range subjects {
		dir := filepath.Join(c.cfg.ArchiveRoot, subject)
		entries, err := listFiles(dir)
		if err != nil {
			continue
		}
		for _, filePath := range entries {
			// 已存在则更新last_seen
			if existing := c.metadata.FindByPath(filePath); existing != nil {
				c.metadata.TouchLastSeen(existing.UUID)
				continue
			}
			// 用户手动放入：分配UUID，文件名提取关键词
			fileName := filepath.Base(filePath)
			extractedKw := extractKeywordsFromName(fileName)
			newUUID := NewUUID()
			c.termDB.AddTerms(subject, extractedKw, newUUID)
			entry := &FileEntry{
				UUID:         newUUID,
				OriginalName: fileName,
				CurrentPath:  filePath,
				Subject:      subject,
				Keywords:     extractedKw,
				CreatedAt:    time.Now(),
				LastSeen:     time.Now(),
				Source:       SourceManual,
			}
			c.metadata.Add(entry)
			c.logFn("[INFO] [手动文件] UUID=%s, 学科=%s, 关键词=%v", newUUID, subject, extractedKw)
		}
	}
}

// extractKeywordsFromName 从文件名提取可能的关键词
// 简单实现：去除后缀、年级、通用词后剩下的中文词
func extractKeywordsFromName(name string) []string {
	// 去除后缀
	for _, ext := range []string{".docx", ".pptx", ".pdf", ".txt", ".zip", ".rar", ".7z", ".xlsx"} {
		if strings.HasSuffix(strings.ToLower(name), ext) {
			name = name[:len(name)-len(ext)]
			break
		}
	}
	// 去除年级词、通用词
	noiseWords := []string{
		"七年级", "八年级", "九年级", "高一", "高二", "高三",
		"课件", "教案", "试卷", "练习", "复习", "期末", "期中", "月考",
		"上册", "下册", "全册",
	}
	cleaned := name
	for _, n := range noiseWords {
		cleaned = strings.ReplaceAll(cleaned, n, "")
	}
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return nil
	}
	// 简单拆分：按中文字符边界
	result := []string{}
	var buf strings.Builder
	for _, r := range cleaned {
		if r >= 0x4e00 && r <= 0x9fff {
			buf.WriteRune(r)
		} else {
			if buf.Len() > 0 {
				result = append(result, buf.String())
				buf.Reset()
			}
		}
	}
	if buf.Len() > 0 {
		result = append(result, buf.String())
	}
	return result
}
