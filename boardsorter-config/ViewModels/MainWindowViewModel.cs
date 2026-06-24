using System;
using System.Collections.ObjectModel;
using System.Threading;
using System.Threading.Tasks;
using Avalonia;
using Avalonia.Styling;
using BoardsorterConfig.Models;
using BoardsorterConfig.Services;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;

namespace BoardsorterConfig.ViewModels;

public partial class MainWindowViewModel : ViewModelBase
{
    private readonly BoardsorterClient _client = new();
    private Timer? _autoRefreshTimer;

    [ObservableProperty]
    private string _connectionStatus = "未连接";

    [ObservableProperty]
    private string _lastActivity = "空闲";

    [ObservableProperty]
    private int _ipcPort;

    [ObservableProperty]
    private string _watchDir = "";

    [ObservableProperty]
    private string _archiveDir = "";

    [ObservableProperty]
    private string _subjectsText = "";

    public ObservableCollection<RuleItem> Rules { get; } = new();

    [ObservableProperty]
    private string _aiEndpoint = "";

    [ObservableProperty]
    private string _aiApiKey = "";

    [ObservableProperty]
    private string _aiModel = "";

    [ObservableProperty]
    private string _aiReasoningLevel = "medium";

    [ObservableProperty]
    private string _aiPrompt = "";

    [ObservableProperty]
    private bool _autoStart;

    [ObservableProperty]
    private bool _startMenuShortcut;

    [ObservableProperty]
    private bool _darkMode;

    partial void OnDarkModeChanged(bool value)
    {
        var app = Application.Current;
        if (app is not null)
        {
            app.RequestedThemeVariant = value ? ThemeVariant.Dark : ThemeVariant.Light;
        }
    }

    [ObservableProperty]
    private bool _autoRefresh;

    partial void OnAutoRefreshChanged(bool value)
    {
        if (value)
        {
            _autoRefreshTimer = new Timer(_ =>
            {
                _ = Task.Run(async () =>
                {
                    await RefreshTermsAsync();
                    await RefreshLogsAsync();
                });
            }, null, TimeSpan.FromSeconds(3), TimeSpan.FromSeconds(5));
        }
        else
        {
            _autoRefreshTimer?.Dispose();
            _autoRefreshTimer = null;
        }
    }

    public ObservableCollection<TermEntry> Terms { get; } = new();

    [ObservableProperty]
    private string _termQuery = "";

    public ObservableCollection<FileMeta> Files { get; } = new();

    public ObservableCollection<LogEntry> Logs { get; } = new();

    public MainWindowViewModel()
    {
        IpcPort = _client.Port;
        _ = RefreshAllAsync();
    }

    [RelayCommand]
    private async Task RefreshAllAsync()
    {
        LastActivity = "正在拉取状态...";
        var ok = await _client.PingAsync();
        ConnectionStatus = ok ? "已连接" : $"未连接 ({_client.LastError})";

        var cfg = await _client.GetConfigAsync();
        if (cfg is not null)
        {
            WatchDir = cfg.Monitor.WatchDir;
            ArchiveDir = cfg.Monitor.ArchiveDir;
            SubjectsText = string.Join(", ", cfg.Monitor.Subjects);
            Rules.Clear();
            foreach (var r in cfg.Monitor.Rules)
            {
                Rules.Add(r);
            }
            AiEndpoint = cfg.AI.Endpoint;
            AiApiKey = cfg.AI.ApiKey;
            AiModel = cfg.AI.Model;
            AiReasoningLevel = cfg.AI.ReasoningLevel;
            AiPrompt = cfg.AI.Prompt;
            AutoStart = cfg.Startup.AutoStart;
            StartMenuShortcut = cfg.Startup.StartMenuShortcut;
            IpcPort = cfg.Startup.IpcPort;
            DarkMode = cfg.Startup.DarkMode;
        }

        await RefreshTermsAsync();
        await RefreshFilesAsync();
        await RefreshLogsAsync();

        LastActivity = $"刷新于 {DateTime.Now:HH:mm:ss}";
    }

    [RelayCommand]
    private async Task RefreshTermsAsync()
    {
        var terms = await _client.SearchTermsAsync(TermQuery);
        Terms.Clear();
        foreach (var t in terms)
        {
            Terms.Add(t);
        }
    }

    [RelayCommand]
    private async Task RefreshFilesAsync()
    {
        var files = await _client.ListFilesAsync();
        Files.Clear();
        foreach (var f in files)
        {
            Files.Add(f);
        }
    }

    [RelayCommand]
    private async Task RefreshLogsAsync()
    {
        var logs = await _client.GetLogsAsync();
        Logs.Clear();
        foreach (var l in logs)
        {
            Logs.Add(l);
        }
    }

    [RelayCommand]
    private async Task SaveConfigAsync()
    {
        var cfg = new ConfigModel
        {
            Monitor = new MonitorConfig
            {
                WatchDir = WatchDir,
                ArchiveDir = ArchiveDir,
                Subjects = new System.Collections.Generic.List<string>(
                    (SubjectsText ?? "").Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries)),
                Rules = new System.Collections.Generic.List<RuleItem>(Rules)
            },
            AI = new AIConfig
            {
                Endpoint = AiEndpoint,
                ApiKey = AiApiKey,
                Model = AiModel,
                ReasoningLevel = AiReasoningLevel,
                Prompt = AiPrompt
            },
            Startup = new StartupConfig
            {
                AutoStart = AutoStart,
                StartMenuShortcut = StartMenuShortcut,
                IpcPort = IpcPort,
                DarkMode = DarkMode
            }
        };
        var ok = await _client.UpdateConfigAsync(cfg);
        LastActivity = ok ? "配置已保存" : $"保存失败: {_client.LastError}";
    }

    [RelayCommand]
    private void AddRule()
    {
        Rules.Add(new RuleItem { Pattern = "新规则", Subject = "未分类", Priority = 0 });
    }

    [RelayCommand]
    private void RemoveRule(RuleItem? item)
    {
        if (item is not null)
        {
            Rules.Remove(item);
        }
    }

    [RelayCommand]
    private async Task CreateStartMenuAsync()
    {
        var ok = await _client.SetSystemStartMenuAsync(true);
        LastActivity = ok ? "已创建开始菜单快捷方式" : $"创建失败: {_client.LastError}";
    }

    [RelayCommand]
    private async Task RemoveStartMenuAsync()
    {
        var ok = await _client.SetSystemStartMenuAsync(false);
        LastActivity = ok ? "已移除开始菜单快捷方式" : $"移除失败: {_client.LastError}";
    }
}
