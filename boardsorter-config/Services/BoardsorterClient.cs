using System;
using System.Collections.Generic;
using System.IO;
using System.Net.Http;
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

    private static int LoadPort()
    {
        try
        {
            var path = Path.Combine(AppContext.BaseDirectory, "data", "ipc.json");
            if (!File.Exists(path))
            {
                // 还可能在 exe 同目录的 data 子目录（相对路径，取决于工作目录）
                var alt = Path.Combine(Directory.GetCurrentDirectory(), "data", "ipc.json");
                if (!File.Exists(alt))
                {
                    return 59812;
                }
                path = alt;
            }
            var json = File.ReadAllText(path);
            using var doc = JsonDocument.Parse(json);
            if (doc.RootElement.TryGetProperty("port", out var p) && p.TryGetInt32(out var v))
            {
                return v;
            }
        }
        catch
        {
            // ignored
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

    public async Task<List<TermEntry>> SearchTermsAsync(string query)
    {
        try
        {
            var url = $"{BaseUrl}/api/terms?keyword={Uri.EscapeDataString(query ?? "")}";
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

    public async Task<List<FileMeta>> ListFilesAsync()
    {
        try
        {
            // /api/files 返回 { total, items }，我们只取 items
            var resp = await _http.GetAsync($"{BaseUrl}/api/files");
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
}
