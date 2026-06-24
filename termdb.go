package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// TermDB 词条数据库
// 替代原来的热词/冷词两套机制
// 单一存储，BM25评分，长时间未命中自动衰减
type TermDB struct {
	mu       sync.RWMutex
	dataDir  string
	path     string
	Subjects []string
	// 词条 -> 学科 -> 词条信息
	// 一个词可同时属于多个学科（跨学科词）
	Terms map[string]map[string]*TermInfo
	logFn  func(string, ...interface{})
}

// TermInfo 词条信息
type TermInfo struct {
	Freq         int       `json:"freq"`          // 词频
	LastMatch    time.Time `json:"last_match"`    // 最后命中时间
	MatchedFiles []string  `json:"matched_files"` // 关联的文件UUID列表
}

// termDBData 持久化数据结构
type termDBData struct {
	Terms map[string]map[string]*TermInfo `json:"terms"`
}

// NewTermDB 创建词条数据库
func NewTermDB(dataDir string, subjects []string, logFn func(string, ...interface{})) (*TermDB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建词库目录失败: %w", err)
	}
	db := &TermDB{
		dataDir:  dataDir,
		path:     filepath.Join(dataDir, "词条库.json"),
		Subjects: subjects,
		Terms:    make(map[string]map[string]*TermInfo),
		logFn:    logFn,
	}
	if err := db.load(); err != nil && !os.IsNotExist(err) {
		logFn("[WARN] 词条库加载失败，已创建空库: %v", err)
	}
	return db, nil
}

// load 从磁盘加载
func (db *TermDB) load() error {
	data, err := os.ReadFile(db.path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	var raw termDBData
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Terms != nil {
		db.Terms = raw.Terms
	}
	return nil
}

// save 保存到磁盘
func (db *TermDB) save() error {
	db.mu.RLock()
	raw := termDBData{Terms: db.Terms}
	data, err := json.MarshalIndent(raw, "", "  ")
	db.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(db.path, data, 0644)
}

// AddTerm 添加词条（AI学习或手动）
func (db *TermDB) AddTerm(subject, term string, fileUUID string) {
	term = strings.TrimSpace(term)
	if term == "" {
		return
	}
	db.mu.Lock()
	if _, ok := db.Terms[term]; !ok {
		db.Terms[term] = make(map[string]*TermInfo)
	}
	if _, ok := db.Terms[term][subject]; !ok {
		db.Terms[term][subject] = &TermInfo{
			Freq:         0,
			LastMatch:    time.Now(),
			MatchedFiles: []string{},
		}
	}
	info := db.Terms[term][subject]
	info.Freq++
	info.LastMatch = time.Now()
	// 关联文件UUID（去重）
	if fileUUID != "" {
		found := false
		for _, u := range info.MatchedFiles {
			if u == fileUUID {
				found = true
				break
			}
		}
		if !found {
			info.MatchedFiles = append(info.MatchedFiles, fileUUID)
		}
	}
	db.mu.Unlock()
	db.save()
}

// AddTerms 批量添加
func (db *TermDB) AddTerms(subject string, terms []string, fileUUID string) {
	for _, t := range terms {
		db.AddTerm(subject, t, fileUUID)
	}
}

// MatchResult 匹配结果
type MatchResult struct {
	Subject     string
	Score       float64
	Keywords    []string // 命中的关键词
	HasMatch    bool
}

// MatchBM25 BM25评分匹配
// 算法说明：
//   - 词频(TF): 词在当前文件名中出现的次数，BM25的k1参数控制饱和度
//   - 逆文档频率(IDF): 词条库中该词关联的学科数越少，权重越高
//     例如"能量"出现在物理和化学两个学科 → IDF低
//     "牛顿定律"只出现在物理 → IDF高
//   - 这样跨学科模糊词天然得分低
func (db *TermDB) MatchBM25(filename string) *MatchResult {
	db.mu.RLock()
	defer db.mu.RUnlock()

	lower := strings.ToLower(filename)
	subjectScores := make(map[string]float64)
	subjectKeywords := make(map[string][]string)

	for term, subjectMap := range db.Terms {
		// 计算词在文件名中的出现次数 (TF)
		tf := countOccurrences(lower, strings.ToLower(term))
		if tf == 0 {
			continue
		}
		// IDF: log( (N - df + 0.5) / (df + 0.5) + 1 )
		// N = 学科总数, df = 该词关联的学科数
		df := float64(len(subjectMap))
		n := float64(len(db.Subjects))
		if n == 0 {
			n = 1
		}
		idf := math.Log((n-df+0.5)/(df+0.5) + 1)
		// BM25 TF部分: (tf * (k1 + 1)) / (tf + k1)
		// k1=1.2是经典默认值
		const k1 = 1.2
		tfScore := (float64(tf) * (k1 + 1)) / (float64(tf) + k1)
		bm25 := idf * tfScore
		// 该词可能属于多个学科，按词频比例分摊
		totalFreq := 0
		for _, info := range subjectMap {
			totalFreq += info.Freq
		}
		if totalFreq == 0 {
			totalFreq = 1
		}
		for subject, info := range subjectMap {
			share := float64(info.Freq) / float64(totalFreq)
			subjectScores[subject] += bm25 * share
			subjectKeywords[subject] = append(subjectKeywords[subject], term)
		}
	}

	if len(subjectScores) == 0 {
		return &MatchResult{HasMatch: false}
	}
	// 找最高分
	type sk struct {
		subject string
		score   float64
	}
	var rankings []sk
	for s, sc := range subjectScores {
		rankings = append(rankings, sk{s, sc})
	}
	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].score > rankings[j].score
	})
	best := rankings[0]
	// 优势比检查：如果最高分与第二名差距太小，认为模糊
	// 这里采用一个简单阈值：最高分需 > 0.1 才有意义
	if best.score < 0.1 {
		return &MatchResult{HasMatch: false}
	}
	return &MatchResult{
		Subject:  best.subject,
		Score:    best.score,
		Keywords: subjectKeywords[best.subject],
		HasMatch: best.score > 0,
	}
}

// Decay 衰减所有词条
// 长时间未命中的词频减半，归零后清理
func (db *TermDB) Decay(maxDaysIdle int) (decayed, removed int) {
	db.mu.Lock()
	defer db.mu.Unlock()
	now := time.Now()
	for term, subjectMap := range db.Terms {
		for subject, info := range subjectMap {
			daysIdle := now.Sub(info.LastMatch).Hours() / 24
			if daysIdle >= float64(maxDaysIdle) {
				if info.Freq > 0 {
					info.Freq = info.Freq / 2
					decayed++
					if info.Freq == 0 {
						delete(db.Terms[term], subject)
						if len(db.Terms[term]) == 0 {
							delete(db.Terms, term)
						}
						removed++
					}
				}
			}
		}
	}
	if decayed > 0 || removed > 0 {
		db.logFn("[INFO] 词条衰减: 衰减 %d 项, 清理 %d 项", decayed, removed)
		go db.save()
	}
	return
}

// GetStats 统计
func (db *TermDB) GetStats() (termCount int) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return len(db.Terms)
}

// GetAllTerms 获取所有词条（用于日志展示）
func (db *TermDB) GetAllTerms() map[string]map[string]*TermInfo {
	db.mu.RLock()
	defer db.mu.RUnlock()
	result := make(map[string]map[string]*TermInfo)
	for k, v := range db.Terms {
		inner := make(map[string]*TermInfo)
		for k2, v2 := range v {
			inner[k2] = v2
		}
		result[k] = inner
	}
	return result
}

// countOccurrences 计算子串在字符串中出现次数
func countOccurrences(s, sub string) int {
	if sub == "" {
		return 0
	}
	count := 0
	idx := 0
	for {
		i := strings.Index(s[idx:], sub)
		if i < 0 {
			break
		}
		count++
		idx += i + len(sub)
	}
	return count
}
