package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// HotWordStore 热词/冷词管理
type HotWordStore struct {
	mu           sync.RWMutex
	dataDir      string
	hotWords     map[string]map[string]string // 学科 -> {词: 最后命中日期}
	coldWords    map[string]map[string]string // 学科 -> {词: 最后命中日期}
	hotPath      string
	coldPath     string
	degradeDays  int // 热词降级天数
	deleteDays   int // 冷词删除天数
	logFn        func(string, ...interface{})
}

// NewHotWordStore 创建词库管理器
func NewHotWordStore(dataDir string, degradeDays, deleteDays int, logFn func(string, ...interface{})) (*HotWordStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建词库目录失败: %w", err)
	}
	store := &HotWordStore{
		dataDir:     dataDir,
		hotWords:    make(map[string]map[string]string),
		coldWords:   make(map[string]map[string]string),
		hotPath:     filepath.Join(dataDir, "热词库.json"),
		coldPath:    filepath.Join(dataDir, "冷词库.json"),
		degradeDays: degradeDays,
		deleteDays:  deleteDays,
		logFn:       logFn,
	}
	if err := store.load(); err != nil {
		store.logFn("[WARN] 词库加载失败，已创建空词库: %v", err)
	}
	return store, nil
}

func (s *HotWordStore) load() error {
	// 加载热词库
	if err := s.loadFile(s.hotPath, &s.hotWords); err != nil && !os.IsNotExist(err) {
		return err
	}
	// 加载冷词库
	if err := s.loadFile(s.coldPath, &s.coldWords); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *HotWordStore) loadFile(path string, target *map[string]map[string]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, target)
}

func (s *HotWordStore) saveHot() error {
	data, err := json.MarshalIndent(s.hotWords, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.hotPath, data, 0644)
}

func (s *HotWordStore) saveCold() error {
	data, err := json.MarshalIndent(s.coldWords, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.coldPath, data, 0644)
}

// MatchHot 用文件名匹配热词，返回匹配的学科和词
func (s *HotWordStore) MatchHot(filename string) (string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lower := strings.ToLower(filename)
	today := time.Now().Format("2006-01-02")

	for subject, words := range s.hotWords {
		for word := range words {
			if strings.Contains(lower, strings.ToLower(word)) {
				// 更新命中时间（写操作需要升级锁，但先读后写在实践中可接受）
				// 这里我们先释放读锁，重新获取写锁
				s.mu.RUnlock()
				s.mu.Lock()
				if _, ok := s.hotWords[subject]; ok {
					if _, ok := s.hotWords[subject][word]; ok {
						s.hotWords[subject][word] = today
						s.saveHot()
					}
				}
				s.mu.Unlock()
				s.mu.RLock()
				return subject, word
			}
		}
	}
	return "", ""
}

// MatchCold 用文件名匹配冷词，返回匹配的学科和词（不更新命中时间）
func (s *HotWordStore) MatchCold(filename string) (string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lower := strings.ToLower(filename)
	for subject, words := range s.coldWords {
		for word := range words {
			if strings.Contains(lower, strings.ToLower(word)) {
				return subject, word
			}
		}
	}
	return "", ""
}

// AddHotWords 添加热词（AI学习）
func (s *HotWordStore) AddHotWords(subject string, keywords []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if _, ok := s.hotWords[subject]; !ok {
		s.hotWords[subject] = make(map[string]string)
	}
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		if _, exists := s.hotWords[subject][kw]; !exists {
			s.logFn("[INFO] 新增热词 [%s] %s", subject, kw)
		}
		s.hotWords[subject][kw] = today
	}
	s.saveHot()
}

// DailyUpgradeDowngrade 每日升降级任务
func (s *HotWordStore) DailyUpgradeDowngrade() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	today := now.Format("2006-01-02")
	changed := false

	// 热词 -> 冷词降级
	for subject, words := range s.hotWords {
		for word, lastHit := range words {
			lastDate, err := time.Parse("2006-01-02", lastHit)
			if err != nil {
				continue
			}
			days := now.Sub(lastDate).Hours() / 24
			if days >= float64(s.degradeDays) {
				// 移出热词
				delete(s.hotWords[subject], word)
				// 加入冷词
				if _, ok := s.coldWords[subject]; !ok {
					s.coldWords[subject] = make(map[string]string)
				}
				s.coldWords[subject][word] = today
				s.logFn("[INFO] 热词降级为冷词 [%s] %s (已%d天未使用)", subject, word, int(days))
				changed = true
			}
		}
		// 清理空学科
		if len(s.hotWords[subject]) == 0 {
			delete(s.hotWords, subject)
		}
	}

	// 冷词删除
	for subject, words := range s.coldWords {
		for word, lastHit := range words {
			lastDate, err := time.Parse("2006-01-02", lastHit)
			if err != nil {
				continue
			}
			days := now.Sub(lastDate).Hours() / 24
			if days >= float64(s.deleteDays) {
				delete(s.coldWords[subject], word)
				s.logFn("[INFO] 冷词已删除 [%s] %s (已%d天未使用)", subject, word, int(days))
				changed = true
			}
		}
		if len(s.coldWords[subject]) == 0 {
			delete(s.coldWords, subject)
		}
	}

	if changed {
		s.saveHot()
		s.saveCold()
	}
}

// GetStats 获取词库统计信息
func (s *HotWordStore) GetStats() (hotCount, coldCount int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, words := range s.hotWords {
		hotCount += len(words)
	}
	for _, words := range s.coldWords {
		coldCount += len(words)
	}
	return
}

// GetHotWords 返回热词库的副本
func (s *HotWordStore) GetHotWords() map[string]map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]map[string]string)
	for k, v := range s.hotWords {
		inner := make(map[string]string)
		for k2, v2 := range v {
			inner[k2] = v2
		}
		result[k] = inner
	}
	return result
}

// GetColdWords 返回冷词库的副本
func (s *HotWordStore) GetColdWords() map[string]map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]map[string]string)
	for k, v := range s.coldWords {
		inner := make(map[string]string)
		for k2, v2 := range v {
			inner[k2] = v2
		}
		result[k] = inner
	}
	return result
}