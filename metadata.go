package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FileSource 文件来源
type FileSource string

const (
	SourceAuto   FileSource = "auto"   // 系统自动归档
	SourceManual FileSource = "manual" // 用户手动放入
)

// FileEntry 单个文件元数据
type FileEntry struct {
	UUID         string     `json:"uuid"`
	OriginalName string     `json:"original_name"`
	CurrentPath  string     `json:"current_path"`
	Subject      string     `json:"subject"`
	Keywords     []string   `json:"keywords"`
	CreatedAt    time.Time  `json:"created_at"`
	LastSeen     time.Time  `json:"last_seen"`
	Source       FileSource `json:"source"`
}

// PendingEntry 待确认条目（文件被改名/移动后暂存）
type PendingEntry struct {
	UUID         string    `json:"uuid"`
	Inode        uint64    `json:"inode"`
	Device       uint64    `json:"device"`
	Subject      string    `json:"subject"`
	DisappearedAt time.Time `json:"disappeared_at"`
}

// FileMetadata 文件元数据存储
type FileMetadata struct {
	mu      sync.RWMutex
	dataDir string
	path    string
	Files   map[string]*FileEntry  // uuid -> entry
	Pending map[string]*PendingEntry // uuid -> pending
}

// NewFileMetadata 创建文件元数据存储
func NewFileMetadata(dataDir string) (*FileMetadata, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建元数据目录失败: %w", err)
	}
	m := &FileMetadata{
		dataDir: dataDir,
		path:    filepath.Join(dataDir, "files.json"),
		Files:   make(map[string]*FileEntry),
		Pending: make(map[string]*PendingEntry),
	}
	if err := m.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return m, nil
}

// load 从磁盘加载
func (m *FileMetadata) load() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	var raw struct {
		Files   map[string]*FileEntry     `json:"files"`
		Pending map[string]*PendingEntry  `json:"pending"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Files != nil {
		m.Files = raw.Files
	}
	if raw.Pending != nil {
		m.Pending = raw.Pending
	}
	return nil
}

// save 保存到磁盘
func (m *FileMetadata) save() error {
	m.mu.RLock()
	raw := struct {
		Files   map[string]*FileEntry     `json:"files"`
		Pending map[string]*PendingEntry  `json:"pending"`
	}{
		Files:   m.Files,
		Pending: m.Pending,
	}
	m.mu.RUnlock()
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0644)
}

// NewUUID 生成新的UUID v4
func NewUUID() string {
	return uuid.New().String()
}

// FindByPath 通过路径查找文件条目
func (m *FileMetadata) FindByPath(path string) *FileEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, entry := range m.Files {
		if entry.CurrentPath == path {
			return entry
		}
	}
	return nil
}

// FindByNameInDir 在指定目录下查找同名文件
func (m *FileMetadata) FindByNameInDir(dir, name string) *FileEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	target := filepath.Join(dir, name)
	for _, entry := range m.Files {
		if entry.CurrentPath == target {
			return entry
		}
	}
	return nil
}

// Add 新增文件记录
func (m *FileMetadata) Add(entry *FileEntry) {
	m.mu.Lock()
	m.Files[entry.UUID] = entry
	m.mu.Unlock()
	m.save()
}

// Update 更新文件记录
func (m *FileMetadata) Update(entry *FileEntry) {
	m.mu.Lock()
	m.Files[entry.UUID] = entry
	m.mu.Unlock()
	m.save()
}

// TouchLastSeen 更新最后看到时间
func (m *FileMetadata) TouchLastSeen(uuidStr string) {
	m.mu.Lock()
	if entry, ok := m.Files[uuidStr]; ok {
		entry.LastSeen = time.Now()
	}
	m.mu.Unlock()
	m.save()
}

// Remove 删除文件记录
func (m *FileMetadata) Remove(uuidStr string) {
	m.mu.Lock()
	delete(m.Files, uuidStr)
	delete(m.Pending, uuidStr)
	m.mu.Unlock()
	m.save()
}

// AddPending 标记为待确认（文件改名/移动后）
func (m *FileMetadata) AddPending(p *PendingEntry) {
	m.mu.Lock()
	m.Pending[p.UUID] = p
	m.mu.Unlock()
	m.save()
}

// MatchPending 在指定目录下查找匹配的pending条目
// 通过inode匹配文件身份
func (m *FileMetadata) MatchPending(fileInfo os.FileInfo) *PendingEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if fileInfo == nil {
		return nil
	}
	// Windows上 inode 不稳定，主要通过文件名/路径做最佳匹配
	for _, p := range m.Pending {
		if p.Subject != "" {
			return p
		}
	}
	return nil
}

// GetStats 统计信息
func (m *FileMetadata) GetStats() (total int, pending int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.Files), len(m.Pending)
}

// CleanupPending 清理超时的待确认条目
func (m *FileMetadata) CleanupPending(maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	removed := 0
	for uuidStr, p := range m.Pending {
		if now.Sub(p.DisappearedAt) > maxAge {
			delete(m.Pending, uuidStr)
			// 同时删除对应的Files记录（既然已消失）
			if entry, ok := m.Files[uuidStr]; ok {
				entry.LastSeen = now
			}
			removed++
		}
	}
	if removed > 0 {
		go m.save()
	}
	return removed
}

// AllEntries 返回所有文件条目
func (m *FileMetadata) AllEntries() []*FileEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*FileEntry, 0, len(m.Files))
	for _, e := range m.Files {
		result = append(result, e)
	}
	return result
}
