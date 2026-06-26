using System;
using System.Collections.ObjectModel;
using System.Diagnostics;
using System.Linq;
using System.Threading.Tasks;
using Avalonia;
using Avalonia.Styling;
using Avalonia.Threading;
using BoardsorterConfig.Models;
using BoardsorterConfig.Services;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;

namespace BoardsorterConfig.ViewModels;

public partial class MainWindowViewModel : ViewModelBase
{
    private readonly BoardsorterClient _client = new();
    private readonly ToastNotifier _toast = new();
    private DispatcherTimer? _autoRefreshTimer;
    private bool _isSaving;
    private bool _isRefreshing;

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

    // Rules - 词条库页面折叠
    public ObservableCollection<RuleItem> Rules { get; } = new();

    [ObservableProperty]
    private RuleItem? _selectedRule;

    [ObservableProperty]
    private string _aiEndpoint = "";

    [ObservableProperty]
    private string _aiApiKey = "";

    [ObservableProperty]
    private string _aiModel = "";

    [ObservableProperty]
    private string _aiReasoningLevel = "medium";

    public string[] AiReasoningLevels { get; } = ["low", "medium", "high"];

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
        Dispatcher.UIThread.Post(() =>
        {
            var app = Application.Current;
            if (app is not null)
            {
                app.RequestedThemeVariant = value ? ThemeVariant.Dark : ThemeVariant.Light;
            }
        });
    }

    // 通知
    [ObservableProperty]
    private bool _notifyEnabled;

    [ObservableProperty]
    private bool _autoRefresh = true;

    partial void OnAutoRefreshChanged(bool value)
    {
        if (value)
            StartAutoRefresh();
        else
            StopAutoRefresh();
    }

    // ========== 词条库 ==========

    [ObservableProperty]
    private ObservableCollection<TermEntry> _terms = new();

    [ObservableProperty]
    private string _termQuery = "";

    [ObservableProperty]
    private string _subjectFilter = "";

    [ObservableProperty]
    private ObservableCollection<string> _subjectOptions = new();

    partial void OnSubjectFilterChanged(string value)
    {
        _ = RefreshTermsAsync();
    }

    /// <summary>
    /// 选中词条 -> 检索关联文件（同时传入科目做精确匹配）
    /// </summary>
    [ObservableProperty]
    private TermEntry? _selectedTerm;

    partial void OnSelectedTermChanged(TermEntry? value)
    {
        if (value != null && !string.IsNullOrEmpty(value.Term))
            _ = SearchFilesByTermAsync(value.Term, value.Subject);
        else
            TermFiles.Clear();
    }

    [ObservableProperty]
    private ObservableCollection<FileMeta> _termFiles = new();

    // ========== 文件元数据 ==========

    [ObservableProperty]
    private ObservableCollection<FileMeta> _files = new();

    [ObservableProperty]
    private string _fileSubjectFilter = "";

    partial void OnFileSubjectFilterChanged(string value)
    {
        _ = RefreshFilesAsync();
    }

    // ========== 日志 ==========

    [ObservableProperty]
    private ObservableCollection<LogEntry> _logs = new();

    public MainWindowViewModel()
    {
        IpcPort = _client.Port;
        _ = RefreshAllAsync();
        if (AutoRefresh)
            StartAutoRefresh();
    }

    private void StartAutoRefresh()
    {
        StopAutoRefresh();
        _autoRefreshTimer = new DispatcherTimer(
            TimeSpan.FromSeconds(5),
            DispatcherPriority.Background,
            async (s, e) =>
            {
                await RefreshTermsAsync();
                await RefreshFilesAsync();
                await RefreshLogsAsync();
                await PollNotificationsAsync();
            });
        _autoRefreshTimer.Start();
    }

    private void StopAutoRefresh()
    {
        _autoRefreshTimer?.Stop();
        _autoRefreshTimer = null;
    }

    [RelayCommand]
    private async Task RefreshAllAsync()
    {
        if (_isRefreshing) return;
        _isRefreshing = true;
        try
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
                NotifyEnabled = false; // 默认关闭，由用户开启

                SubjectOptions.Clear();
                SubjectOptions.Add(""); // 空=全部
                foreach (var s in cfg.Monitor.Subjects)
                {
                    if (!SubjectOptions.Contains(s))
                        SubjectOptions.Add(s);
                }
            }

            await RefreshTermsAsync();
            await RefreshFilesAsync();
            await RefreshLogsAsync();

            LastActivity = $"刷新于 {DateTime.Now:HH:mm:ss}";
        }
        finally
        {
            _isRefreshing = false;
        }
    }

    [RelayCommand]
    private async Task RefreshTermsAsync()
    {
        var terms = await _client.SearchTermsAsync(TermQuery, SubjectFilter);
        Terms = new ObservableCollection<TermEntry>(terms);
    }

    [RelayCommand]
    private async Task RefreshFilesAsync()
    {
        var files = await _client.ListFilesAsync(FileSubjectFilter);
        Files = new ObservableCollection<FileMeta>(files);
    }

    [RelayCommand]
    private async Task RefreshLogsAsync()
    {
        var logs = await _client.GetLogsAsync();
        Logs = new ObservableCollection<LogEntry>(logs);
    }

    /// <summary>
    /// 根据选中词条检索关联文件，同时传入term和subject做精确匹配
    /// </summary>
    private async Task SearchFilesByTermAsync(string term, string subject)
    {
        var files = await _client.SearchFilesByTermAsync(term, subject);
        TermFiles = new ObservableCollection<FileMeta>(files);
    }

    /// <summary>
    /// 打开文件（系统默认程序）
    /// </summary>
    [RelayCommand]
    private void OpenFile(string? path)
    {
        if (string.IsNullOrEmpty(path)) return;
        try
        {
            using var proc = new Process();
            proc.StartInfo = new ProcessStartInfo(path)
            {
                UseShellExecute = true
            };
            proc.Start();
        }
        catch (Exception ex)
        {
            LastActivity = $"打开文件失败: {ex.Message}";
        }
    }

    /// <summary>
    /// 轮询 Go 端通知队列，通过 Windows Toast 显示
    /// </summary>
    private async Task PollNotificationsAsync()
    {
        if (!NotifyEnabled) return;
        var notifications = await _client.GetClassIslandNotificationsAsync();
        foreach (var n in notifications)
        {
            _toast.SendNotification(n.FileName, n.Subject);
        }
    }

    [RelayCommand]
    private async Task SaveConfigAsync()
    {
        if (_isSaving) return;
        _isSaving = true;
        try
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
        finally
        {
            _isSaving = false;
        }
    }

    [RelayCommand]
    private void AddRule()
    {
        Rules.Add(new RuleItem { Pattern = "新规则", Subject = "未分类", Priority = 0 });
    }

    [RelayCommand]
    private void RemoveSelectedRule()
    {
        if (SelectedRule is not null)
        {
            Rules.Remove(SelectedRule);
            SelectedRule = null;
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