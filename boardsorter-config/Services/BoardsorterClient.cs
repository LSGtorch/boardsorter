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
                return 59812;
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

    private string BaseUrl => $"http://127.0.0.1:{_port}";

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
            var resp = await _http.GetAsync($"{BaseUrl}/api/config");
            if (!resp.IsSuccessStatusCode)
            {
                _lastError = $"HTTP {(int)resp.StatusCode}";
                return new ConfigModel();
            }
            var body = await resp.Content.ReadAsStringAsync();
            var cfg = JsonSerializer.Deserialize<ConfigModel>(body, new JsonSerializerOptions
            {
                PropertyNameCaseInsensitive = true
            });
            _lastError = "";
            return cfg ?? new ConfigModel();
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
            var list = JsonSerializer.Deserialize<List<TermEntry>>(body, new JsonSerializerOptions
            {
                PropertyNameCaseInsensitive = true
            });
            _lastError = "";
            return list ?? new List<TermEntry>();
        }
        catch (Exception ex)
        {
            _lastError = ex.Message;
            return new List<TermEntry>();
        }
    }

    public async Task<List<FileMeta>> ListFilesAsync()
    {
        try
        {
            var resp = await _http.GetAsync($"{BaseUrl}/api/files");
            if (!resp.IsSuccessStatusCode)
            {
                _lastError = $"HTTP {(int)resp.StatusCode}";
                return new List<FileMeta>();
            }
            var body = await resp.Content.ReadAsStringAsync();
            var list = JsonSerializer.Deserialize<List<FileMeta>>(body, new JsonSerializerOptions
            {
                PropertyNameCaseInsensitive = true
            });
            _lastError = "";
            return list ?? new List<FileMeta>();
        }
        catch (Exception ex)
        {
            _lastError = ex.Message;
            return new List<FileMeta>();
        }
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
            var list = JsonSerializer.Deserialize<List<LogEntry>>(body, new JsonSerializerOptions
            {
                PropertyNameCaseInsensitive = true
            });
            _lastError = "";
            return list ?? new List<LogEntry>();
        }
        catch (Exception ex)
        {
            _lastError = ex.Message;
            return new List<LogEntry>();
        }
    }
}
