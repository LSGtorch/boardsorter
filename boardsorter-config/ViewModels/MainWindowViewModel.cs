using System;
using System.Collections.ObjectModel;
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
    private readonly ClassIslandIpcBridge _ciBridge = new();
    private DispatcherTimer? _autoRefreshTimer;

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

    // Rules - moved to 词条库页面
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
        // 确保在 UI 线程切换主题
        Dispatcher.UIThread.Post(() =>
        {
            var app = Application.Current;
            if (app is not null)
            {
                app.RequestedThemeVariant = value ? ThemeVariant.Dark : ThemeVariant.Light;
            }
        });
    }

    // ClassIsland 通知
    [ObservableProperty]
    private string _classIslandStatus = "";

    [ObservableProperty]
    private bool _classIslandNotifyEnabled;

    [ObservableProperty]
    private bool _autoRefresh = true; // 默认启用自动刷新

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

    /// <summary>
    /// 科目筛选（空=全部）
    /// </summary>
    [ObservableProperty]
    private string _subjectFilter = "";

    /// <summary>
    /// 可选科目列表（从配置加载）
    /// </summary>
    [ObservableProperty]
    private ObservableCollection<string> _subjectOptions = new();

    partial void OnSubjectFilterChanged(string value)
    {
        _ = RefreshTermsAsync();
    }

    /// <summary>
    /// 选中词条 → 检索关联文件
    /// </summary>
    [ObservableProperty]
    private TermEntry? _selectedTerm;

    partial void OnSelectedTermChanged(TermEntry? value)
    {
        if (value != null)
            _ = SearchFilesByTermAsync(value.Term);
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
        // 自动刷新默认启用，启动定时器
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
                await PollClassIslandNotificationsAsync();
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
            ClassIslandNotifyEnabled = cfg.ClassIsland.NotifyEnabled;

            // 更新科目列表
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

        // 尝试连接 ClassIsland
        if (ClassIslandNotifyEnabled)
        {
            await _ciBridge.ConnectAsync();
            ClassIslandStatus = _ciBridge.Connected ? "已连接 ClassIsland" : $"ClassIsland: {_ciBridge.LastError}";
        }
        else
        {
            ClassIslandStatus = "通知未启用";
        }

        LastActivity = $"刷新于 {DateTime.Now:HH:mm:ss}";
    }

    [RelayCommand]
    private async Task RefreshTermsAsync()
    {
        var terms = await _client.SearchTermsAsync(TermQuery, SubjectFilter);
        // 替换整个集合以保持 DataGrid 滚动位置稳定
        Terms = new ObservableCollection<TermEntry>(terms);
    }

    [RelayCommand]
    private async Task RefreshFilesAsync()
    {
        var files = await _client.ListFilesAsync(FileSubjectFilter);
        // 替换整个集合以保持 DataGrid 滚动位置稳定
        Files = new ObservableCollection<FileMeta>(files);
    }

    [RelayCommand]
    private async Task RefreshLogsAsync()
    {
        var logs = await _client.GetLogsAsync();
        Logs = new ObservableCollection<LogEntry>(logs);
    }

    /// <summary>
    /// 根据选中的词条检索关联文件
    /// </summary>
    private async Task SearchFilesByTermAsync(string term)
    {
        if (string.IsNullOrEmpty(term))
        {
            TermFiles.Clear();
            return;
        }
        var files = await _client.SearchFilesByTermAsync(term);
        TermFiles = new ObservableCollection<FileMeta>(files);
    }

    /// <summary>
    /// 轮询 Go 端通知队列，通过 Windows Toast 显示
    /// </summary>
    private async Task PollClassIslandNotificationsAsync()
    {
        if (!ClassIslandNotifyEnabled) return;

        var notifications = await _client.GetClassIslandNotificationsAsync();
        foreach (var n in notifications)
        {
            _ciBridge.SendNotification(n.FileName, n.Subject);
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
            },
            ClassIsland = new ClassIslandConfig
            {
                NotifyEnabled = ClassIslandNotifyEnabled,
                NotifyURL = "",
                NotifyTemplate = ""
            }
        };
        var ok = await _client.UpdateConfigAsync(cfg);
        LastActivity = ok ? "配置已保存" : $"保存失败: {_client.LastError}";

        // 保存后同步 ClassIsland 连接状态
        if (ClassIslandNotifyEnabled)
        {
            await _ciBridge.ConnectAsync();
            ClassIslandStatus = _ciBridge.Connected ? "已连接 ClassIsland" : $"ClassIsland: {_ciBridge.LastError}";
        }
        else
        {
            _ciBridge.Dispose();
            ClassIslandStatus = "通知未启用";
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