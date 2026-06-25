using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Net.Http;
using System.Reflection;
using System.Text;
using System.Text.Json;
using System.Threading.Tasks;
using BoardsorterConfig.Models;

namespace BoardsorterConfig.Services;

public class BoardsorterClient
{
    private readonly HttpClient _http;
    private int _port;
    private string _lastError = "";
    private static readonly string _exeDir = GetExeDirectory();

    public string LastError => _lastError;
    public int Port => _port;

    public BoardsorterClient()
    {
        _http = new HttpClient
        {
            Timeout = TimeSpan.FromSeconds(5)
        };
        _port = LoadPort();
    }

    /// <summary>
    /// 获取 exe 实际所在目录（兼容单文件发布）
    /// </summary>
    private static string GetExeDirectory()
    {
        // 优先用 ProcessPath（单文件发布也正确）
        var p = Environment.ProcessPath;
        if (!string.IsNullOrEmpty(p))
        {
            return Path.GetDirectoryName(p) ?? ".";
        }
        // 回退
        return AppContext.BaseDirectory;
    }

    private static int LoadPort()
    {
        // 按优先级查找 data/ipc.json
        var searchPaths = new[]
        {
            Path.Combine(_exeDir, "data", "ipc.json"),
            Path.Combine(Directory.GetCurrentDirectory(), "data", "ipc.json"),
        };

        foreach (var path in searchPaths)
        {
            try
            {
                if (!File.Exists(path)) continue;
                var json = File.ReadAllText(path);
                using var doc = JsonDocument.Parse(json);
                if (doc.RootElement.TryGetProperty("port", out var p) && p.TryGetInt32(out var v))
                {
                    return v;
                }
            }
            catch { /* try next */ }
        }

        // 如果 ipc.json 还没生成，尝试候选端口 59812-59820
        var candidatePorts = new[] { 59812, 59813, 59814, 59815, 59816, 59817, 59818, 59819, 59820 };
        foreach (var port in candidatePorts)
        {
            try
            {
                using var c = new System.Net.Sockets.TcpClient();
                c.ConnectAsync("127.0.0.1", port).Wait(TimeSpan.FromMilliseconds(300));
                return port;
            }
            catch { /* try next */ }
        }

        return 59812;
    }

    // 刷新端口（Go 端启动后 ipc.json 才写入，可能启动时还没生成）
    public void RefreshPort()
    {
        _port = LoadPort();
    }

    private string BaseUrl => $"http://127.0.0.1:{_port}";

    // 通用 API 响应包装：Go 端 writeOK 返回 { ok, data } 结构
    private class ApiResponse<T>
    {
        public bool Ok { get; set; }
        public T? Data { get; set; }
        public string? Error { get; set; }
    }

    private async Task<T?> GetDataAsync<T>(string url)
    {
        var resp = await _http.GetAsync(url);
        if (!resp.IsSuccessStatusCode)
        {
            _lastError = $"HTTP {(int)resp.StatusCode}";
            return default;
        }
        var body = await resp.Content.ReadAsStringAsync();
        var wrapper = JsonSerializer.Deserialize<ApiResponse<T>>(body, new JsonSerializerOptions
        {
            PropertyNameCaseInsensitive = true
        });
        if (wrapper == null || !wrapper.Ok)
        {
            _lastError = wrapper?.Error ?? "API 返回失败";
            return default;
        }
        _lastError = "";
        return wrapper.Data;
    }

    public async Task<bool> PingAsync()
    {
        try
        {
            var resp = await _http.GetAsync($"{BaseUrl}/api/ping");
            _lastError = resp.IsSuccessStatusCode ? "" : $"HTTP {(int)resp.StatusCode}";
            return resp.IsSuccessStatusCode;
        }
        catch (Exception ex)
        {
            _lastError = ex.Message;
            return false;
        }
    }

    public async Task<ConfigModel> GetConfigAsync()
    {
        try
        {
            var data = await GetDataAsync<ConfigModel>($"{BaseUrl}/api/config");
            return data ?? new ConfigModel();
        }
        catch (Exception ex)
        {
            _lastError = ex.Message;
            return new ConfigModel();
        }
    }

    public async Task<bool> UpdateConfigAsync(ConfigModel cfg)
    {
        try
        {
            var json = JsonSerializer.Serialize(cfg, new JsonSerializerOptions
            {
                WriteIndented = false
            });
            var content = new StringContent(json, Encoding.UTF8, "application/json");
            var resp = await _http.PostAsync($"{BaseUrl}/api/config", content);
            _lastError = resp.IsSuccessStatusCode ? "" : $"HTTP {(int)resp.StatusCode}";
            return resp.IsSuccessStatusCode;
        }
        catch (Exception ex)
        {
            _lastError = ex.Message;
            return false;
        }
    }

    public async Task<List<TermEntry>> SearchTermsAsync(string query, string subject = "")
    {
        try
        {
            var url = $"{BaseUrl}/api/terms?keyword={Uri.EscapeDataString(query ?? "")}";
            if (!string.IsNullOrEmpty(subject))
                url += $"&subject={Uri.EscapeDataString(subject)}";
            var resp = await _http.GetAsync(url);
            if (!resp.IsSuccessStatusCode)
            {
                _lastError = $"HTTP {(int)resp.StatusCode}";
                return new List<TermEntry>();
            }
            var body = await resp.Content.ReadAsStringAsync();
            var wrapper = JsonSerializer.Deserialize<ApiResponse<TermListResponse>>(body, new JsonSerializerOptions
            {
                PropertyNameCaseInsensitive = true
            });
            _lastError = "";
            return wrapper?.Data?.Items ?? new List<TermEntry>();
        }
        catch (Exception ex)
        {
            _lastError = ex.Message;
            return new List<TermEntry>();
        }
    }

    private class TermListResponse
    {
        public List<TermEntry> Items { get; set; } = new();
        public int Total { get; set; }
    }

    public async Task<List<FileMeta>> ListFilesAsync(string subject = "", string term = "")
    {
        try
        {
            // /api/files 返回 { total, items }，我们只取 items
            var url = $"{BaseUrl}/api/files";
            var query = new List<string>();
            if (!string.IsNullOrEmpty(subject))
                query.Add($"subject={Uri.EscapeDataString(subject)}");
            if (!string.IsNullOrEmpty(term))
                query.Add($"term={Uri.EscapeDataString(term)}");
            if (query.Count > 0)
                url += "?" + string.Join("&", query);
            var resp = await _http.GetAsync(url);
            if (!resp.IsSuccessStatusCode)
            {
                _lastError = $"HTTP {(int)resp.StatusCode}";
                return new List<FileMeta>();
            }
            var body = await resp.Content.ReadAsStringAsync();
            var wrapper = JsonSerializer.Deserialize<ApiResponse<FileListResponse>>(body, new JsonSerializerOptions
            {
                PropertyNameCaseInsensitive = true
            });
            _lastError = "";
            return wrapper?.Data?.Items ?? new List<FileMeta>();
        }
        catch (Exception ex)
        {
            _lastError = ex.Message;
            return new List<FileMeta>();
        }
    }

    public async Task<List<FileMeta>> SearchFilesByTermAsync(string term)
    {
        return await ListFilesAsync("", term);
    }

    private class FileListResponse
    {
        public List<FileMeta> Items { get; set; } = new();
        public int Total { get; set; }
    }

    public async Task<List<LogEntry>> GetLogsAsync(int limit = 200)
    {
        try
        {
            var resp = await _http.GetAsync($"{BaseUrl}/api/logs?n={limit}");
            if (!resp.IsSuccessStatusCode)
            {
                _lastError = $"HTTP {(int)resp.StatusCode}";
                return new List<LogEntry>();
            }
            var body = await resp.Content.ReadAsStringAsync();
            var wrapper = JsonSerializer.Deserialize<ApiResponse<LogListResponse>>(body, new JsonSerializerOptions
            {
                PropertyNameCaseInsensitive = true
            });
            _lastError = "";
            var lines = wrapper?.Data?.Items ?? new List<string>();
            var result = new List<LogEntry>();
            foreach (var line in lines)
            {
                // 日志格式：[时间] [级别] 消息
                var entry = new LogEntry { Message = line, Level = "INFO", Time = "" };
                if (line.StartsWith('[') && line.Contains(']'))
                {
                    var first = line.IndexOf(']');
                    entry.Time = line.Substring(1, first - 1);
                    var rest = line.Substring(first + 1).Trim();
                    if (rest.StartsWith('[') && rest.Contains(']'))
                    {
                        var second = rest.IndexOf(']');
                        entry.Level = rest.Substring(1, second - 1);
                        entry.Message = rest.Substring(second + 1).Trim();
                    }
                    else
                    {
                        entry.Message = rest;
                    }
                }
                result.Add(entry);
            }
            return result;
        }
        catch (Exception ex)
        {
            _lastError = ex.Message;
            return new List<LogEntry>();
        }
    }

    private class LogListResponse
    {
        public List<string> Items { get; set; } = new();
        public int Count { get; set; }
    }

    public async Task<bool> SetSystemStartMenuAsync(bool enabled)
    {
        try
        {
            var json = JsonSerializer.Serialize(new { enabled });
            var content = new StringContent(json, Encoding.UTF8, "application/json");
            var resp = await _http.PostAsync($"{BaseUrl}/api/system/startmenu", content);
            _lastError = resp.IsSuccessStatusCode ? "" : $"HTTP {(int)resp.StatusCode}";
            return resp.IsSuccessStatusCode;
        }
        catch (Exception ex)
        {
            _lastError = ex.Message;
            return false;
        }
    }

    public async Task<List<ClassIslandNotification>> GetClassIslandNotificationsAsync()
    {
        try
        {
            var data = await GetDataAsync<List<ClassIslandNotification>>($"{BaseUrl}/api/classisland/notifications");
            return data ?? new List<ClassIslandNotification>();
        }
        catch (Exception ex)
        {
            _lastError = ex.Message;
            return new List<ClassIslandNotification>();
        }
    }
}
