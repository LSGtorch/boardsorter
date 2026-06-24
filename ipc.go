package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 全局引用，由 StartIPCServer 在启动时赋值；其他模块也可直接访问
var (
	ipcConfig     *Config
	ipcTermDB     *TermDB
	ipcMetadata   *FileMetadata
	ipcClassifier *Classifier
	ipcLog        *Logger
	ipcMonitor    *Monitor
	ipcDelDeleter *DelayedDeleter
)

// 辅助函数占位：实际实现由 main.go 在 init() 中赋值
//   - triggerManualScan: 触发一次手动扫描，返回扫描到的文件数
//   - onStop:           IPC 服务收到 /api/stop 时调用，决定 main.go 的清理顺序
var (
	triggerManualScan = func() (int, error) { return 0, fmt.Errorf("triggerManualScan 未初始化") }
	onStop            = func() {}
)

// ipcServer 当前 HTTP Server 引用，供 onStop 回调优雅关闭
var ipcServer *http.Server

// 端口候选：优先 59812，被占则顺序尝试 59813-59820
var ipcPortList = []int{59812, 59813, 59814, 59815, 59816, 59817, 59818, 59819, 59820}

// ============== 通用工具 ==============

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeOK 写入成功响应
func writeOK(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "data": data})
}

// writeErr 写入失败响应
func writeErr(w http.ResponseWriter, status int, errMsg string) {
	writeJSON(w, status, map[string]interface{}{"ok": false, "error": errMsg})
}

// corsHandler 给 handler 加上 CORS 头并处理 OPTIONS 预检
func corsHandler(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h(w, r)
	}
}

// ============== 启动入口 ==============

// StartIPCServer 启动 IPC HTTP 服务
// 监听 127.0.0.1 上的 59812-59820 空闲端口；返回实际绑定端口
func StartIPCServer(
	cfg *Config,
	termDB *TermDB,
	metadata *FileMetadata,
	classifier *Classifier,
	log *Logger,
	monitor *Monitor,
	deleter *DelayedDeleter,
	onStopCb func(),
) (int, error) {
	ipcConfig = cfg
	ipcTermDB = termDB
	ipcMetadata = metadata
	ipcClassifier = classifier
	ipcLog = log
	ipcMonitor = monitor
	ipcDelDeleter = deleter
	if onStopCb != nil {
		onStop = onStopCb
	}

	// 路由注册
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ping", corsHandler(handlePing))
	mux.HandleFunc("/api/status", corsHandler(handleStatus))
	mux.HandleFunc("/api/config", corsHandler(handleConfig))
	mux.HandleFunc("/api/terms", corsHandler(handleTerms))
	mux.HandleFunc("/api/files", corsHandler(handleFilesList))
	mux.HandleFunc("/api/files/", corsHandler(handleFileDetail))
	mux.HandleFunc("/api/scan", corsHandler(handleScan))
	mux.HandleFunc("/api/logs", corsHandler(handleLogs))
	mux.HandleFunc("/api/stats", corsHandler(handleStats))
	mux.HandleFunc("/api/decay", corsHandler(handleDecay))
	mux.HandleFunc("/api/stop", corsHandler(handleStop))
	mux.HandleFunc("/api/system/startmenu", corsHandler(handleSystemStartMenu))

	// 顺序尝试候选端口
	var listener net.Listener
	var chosenPort int
	var lastErr error
	for _, p := range ipcPortList {
		addr := fmt.Sprintf("127.0.0.1:%d", p)
		l, err := net.Listen("tcp", addr)
		if err != nil {
			lastErr = err
			continue
		}
		listener = l
		chosenPort = p
		break
	}
	if listener == nil {
		return 0, fmt.Errorf("IPC 端口 59812-59820 全部被占用: %w", lastErr)
	}

	// 把端口信息写入 data/ipc.json（如果 dataDir 存在）
	writeIPCInfo(chosenPort)

	ipcServer = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	if ipcLog != nil {
		ipcLog.Info("IPC HTTP 服务已启动: http://127.0.0.1:%d", chosenPort)
	}

	// 异步 Serve；不影响调用方
	go func() {
		if err := ipcServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			if ipcLog != nil {
				ipcLog.Error("IPC HTTP 服务异常退出: %v", err)
			}
		}
	}()

	return chosenPort, nil
}

// writeIPCInfo 把端口信息写到 data/ipc.json（如果 data 目录存在）
func writeIPCInfo(port int) {
	dataDir := "data"
	if info, err := os.Stat(dataDir); err != nil || !info.IsDir() {
		return
	}
	payload := map[string]int{"port": port}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dataDir, "ipc.json"), data, 0644)
}

// ============== 各个 endpoint ==============

// GET /api/ping
func handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeOK(w, map[string]bool{"pong": true})
}

// GET /api/status
func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	status := map[string]interface{}{
		"app_name":   appDisplayName,
		"version":    appVersion,
		"running":    true,
		"checked_at": time.Now().Format(time.RFC3339),
	}
	if ipcConfig != nil {
		status["watch_folder"] = ipcConfig.WatchFolder
		status["archive_root"] = ipcConfig.ArchiveRoot
		status["subjects"] = ipcConfig.SubjectFolders
		status["irrelevant_folder"] = ipcConfig.IrrelevantFolder
		status["uncertain_folder"] = ipcConfig.UncertainFolder
		status["log_folder"] = ipcConfig.LogFolder
	}
	if ipcTermDB != nil {
		status["term_count"] = ipcTermDB.GetStats()
	}
	if ipcMetadata != nil {
		total, pending := ipcMetadata.GetStats()
		status["file_count"] = total
		status["pending_count"] = pending
	}
	if ipcMonitor != nil {
		status["monitor_running"] = true
	}
	if ipcDelDeleter != nil {
		status["deleter_running"] = true
	}
	writeOK(w, status)
}

// GET/POST /api/config
func handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if ipcConfig == nil {
			writeErr(w, http.StatusInternalServerError, "config 未初始化")
			return
		}
		writeOK(w, sanitizeConfig(ipcConfig))
	case http.MethodPost:
		if ipcConfig == nil {
			writeErr(w, http.StatusInternalServerError, "config 未初始化")
			return
		}
		var newCfg Config
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			writeErr(w, http.StatusBadRequest, "JSON 解析失败: "+err.Error())
			return
		}
		// 客户端若传 "***" 表示保留原 APIKey
		if newCfg.APIKey == "" || newCfg.APIKey == "***" {
			newCfg.APIKey = ipcConfig.APIKey
		}
		// 派生字段重新计算
		newCfg.ReadableExtList = parseExtList(newCfg.ReadableExts)
		newCfg.ArchiveExtList = parseExtList(newCfg.ArchiveExts)
		if err := newCfg.Validate(); err != nil {
			writeErr(w, http.StatusBadRequest, "配置无效: "+err.Error())
			return
		}

		oldWatch := ipcConfig.WatchFolder
		// 整体替换内存中的 cfg
		*ipcConfig = newCfg

		// 若监控目录变化则重启 Monitor
		if newCfg.WatchFolder != oldWatch && ipcLog != nil {
			restartMonitor(ipcConfig)
		}
		if ipcLog != nil {
			ipcLog.Info("配置已通过 IPC 热重载")
		}
		writeOK(w, sanitizeConfig(ipcConfig))
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// sanitizeConfig 复制配置并把敏感字段替换为 ***
func sanitizeConfig(c *Config) Config {
	clone := *c
	if clone.APIKey != "" {
		clone.APIKey = "***"
	}
	return clone
}

// restartMonitor 用最新 cfg 的 WatchFolder 重新创建 Monitor
func restartMonitor(cfg *Config) {
	if ipcMonitor != nil {
		ipcMonitor.Stop()
	}
	if ipcClassifier == nil || ipcLog == nil {
		return
	}
	newMon, err := NewMonitor(cfg.WatchFolder, ipcClassifier.ClassifyFile, ipcLog.Raw)
	if err != nil {
		ipcLog.Error("重启文件监控失败: %v", err)
		return
	}
	newMon.Start()
	ipcMonitor = newMon
	ipcLog.Info("监控目录已切换为: %s", cfg.WatchFolder)
}

// GET /api/terms?keyword=xxx&subject=xxx&limit=50
func handleTerms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if ipcTermDB == nil {
		writeErr(w, http.StatusInternalServerError, "termdb 未初始化")
		return
	}
	q := r.URL.Query()
	keyword := strings.TrimSpace(q.Get("keyword"))
	subject := strings.TrimSpace(q.Get("subject"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	all := ipcTermDB.GetAllTerms()
	type termItem struct {
		Term     string                 `json:"term"`
		Subjects map[string]interface{} `json:"subjects"`
	}
	results := make([]termItem, 0, len(all))
	kwLower := strings.ToLower(keyword)
	for term, subjectMap := range all {
		if keyword != "" && !strings.Contains(strings.ToLower(term), kwLower) {
			continue
		}
		if subject != "" {
			if _, ok := subjectMap[subject]; !ok {
				continue
			}
		}
		subjects := make(map[string]interface{}, len(subjectMap))
		for s, info := range subjectMap {
			subjects[s] = map[string]interface{}{
				"freq":       info.Freq,
				"last_match": info.LastMatch,
			}
		}
		results = append(results, termItem{Term: term, Subjects: subjects})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Term < results[j].Term })
	if len(results) > limit {
		results = results[:limit]
	}
	writeOK(w, map[string]interface{}{
		"total": len(results),
		"items": results,
	})
}

// GET /api/files?subject=xxx&limit=100&offset=0
func handleFilesList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if ipcMetadata == nil {
		writeErr(w, http.StatusInternalServerError, "metadata 未初始化")
		return
	}
	q := r.URL.Query()
	subject := strings.TrimSpace(q.Get("subject"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	offset, _ := strconv.Atoi(q.Get("offset"))
	if offset < 0 {
		offset = 0
	}

	all := ipcMetadata.AllEntries()
	filtered := make([]*FileEntry, 0, len(all))
	for _, e := range all {
		if subject != "" && e.Subject != subject {
			continue
		}
		filtered = append(filtered, e)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := len(filtered)
	if offset >= total {
		writeOK(w, map[string]interface{}{"total": total, "items": []interface{}{}})
		return
	}
	end := offset + limit
	if end > total {
		end = total
	}
	writeOK(w, map[string]interface{}{"total": total, "items": filtered[offset:end]})
}

// GET /api/files/{uuid}
func handleFileDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if ipcMetadata == nil {
		writeErr(w, http.StatusInternalServerError, "metadata 未初始化")
		return
	}
	uuid := strings.TrimPrefix(r.URL.Path, "/api/files/")
	uuid = strings.TrimSuffix(uuid, "/")
	if uuid == "" {
		writeErr(w, http.StatusBadRequest, "uuid 不能为空")
		return
	}
	ipcMetadata.mu.RLock()
	entry, ok := ipcMetadata.Files[uuid]
	ipcMetadata.mu.RUnlock()
	if !ok {
		writeErr(w, http.StatusNotFound, "未找到该 uuid 对应的文件")
		return
	}
	writeOK(w, entry)
}

// POST /api/scan
func handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	count, err := triggerManualScan()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]int{"scanned": count})
}

// GET /api/logs?n=50
func handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if ipcLog == nil {
		writeErr(w, http.StatusInternalServerError, "logger 未初始化")
		return
	}
	q := r.URL.Query()
	n, _ := strconv.Atoi(q.Get("n"))
	if n <= 0 {
		n = 50
	}

	logPath := filepath.Join(ipcLog.logDir, fmt.Sprintf("boardsorter-%s.log", time.Now().Format("2006-01-02")))
	data, err := os.ReadFile(logPath)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "读取日志失败: "+err.Error())
		return
	}
	lines := strings.Split(string(data), "\n")
	// 去除末尾空行
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	writeOK(w, map[string]interface{}{
		"path":  logPath,
		"count": len(lines),
		"items": lines,
	})
}

// GET /api/stats
func handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	stats := map[string]interface{}{}
	if ipcTermDB != nil {
		stats["term_count"] = ipcTermDB.GetStats()
	}
	if ipcMetadata != nil {
		total, pending := ipcMetadata.GetStats()
		stats["file_count"] = total
		stats["pending_count"] = pending
	}
	if ipcMetadata != nil {
		all := ipcMetadata.AllEntries()
		bySubject := make(map[string]int)
		for _, e := range all {
			bySubject[e.Subject]++
		}
		stats["by_subject"] = bySubject
	}
	writeOK(w, stats)
}

// POST /api/decay
func handleDecay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if ipcTermDB == nil {
		writeErr(w, http.StatusInternalServerError, "termdb 未初始化")
		return
	}
	maxDays := 30
	if ipcConfig != nil && ipcConfig.TermMaxIdleDays > 0 {
		maxDays = ipcConfig.TermMaxIdleDays
	}
	decayed, removed := ipcTermDB.Decay(maxDays)
	writeOK(w, map[string]int{
		"decayed":  decayed,
		"removed":  removed,
		"max_days": maxDays,
	})
}

// POST /api/stop  -> 先返回 OK，再异步触发 onStop
func handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeOK(w, map[string]bool{"stopping": true})
	// 异步触发，确保响应先写回客户端
	go func() {
		time.Sleep(100 * time.Millisecond)
		onStop()
	}()
}

// POST /api/system/startmenu
// 请求 body: {"enabled": true} 或 {"enabled": false}
// enabled=true  时调用 CreateStartMenuShortcuts（已存在则跳过）
// enabled=false 时调用 RemoveStartMenuShortcuts（幂等）
func handleSystemStartMenu(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON 解析失败: "+err.Error())
		return
	}

	if req.Enabled {
		// 已存在则跳过 -> 幂等
		if hasStartMenuShortcuts(appDisplayName) {
			if ipcLog != nil {
				ipcLog.Info("[IPC] 开始菜单快捷方式已存在，跳过创建")
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "enabled": true, "existed": true})
			return
		}
		execPath, err := os.Executable()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "获取 exe 路径失败: "+err.Error())
			return
		}
		if err := CreateStartMenuShortcuts(execPath, appDisplayName); err != nil {
			if ipcLog != nil {
				ipcLog.Warn("[IPC] 创建开始菜单快捷方式失败: %v", err)
			}
			writeErr(w, http.StatusInternalServerError, "创建开始菜单快捷方式失败: "+err.Error())
			return
		}
		if ipcLog != nil {
			ipcLog.Info("[IPC] 已创建开始菜单快捷方式: %s", execPath)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "enabled": true, "existed": false})
		return
	}

	// enabled=false
	if err := RemoveStartMenuShortcuts(appDisplayName); err != nil {
		if ipcLog != nil {
			ipcLog.Warn("[IPC] 移除开始菜单快捷方式失败: %v", err)
		}
		writeErr(w, http.StatusInternalServerError, "移除开始菜单快捷方式失败: "+err.Error())
		return
	}
	if ipcLog != nil {
		ipcLog.Info("[IPC] 已移除开始菜单快捷方式")
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "enabled": false})
}
