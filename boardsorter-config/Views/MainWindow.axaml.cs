using System;
using System.Threading.Tasks;
using Avalonia.Controls;
using Avalonia.Threading;
using BoardsorterConfig.Services;

namespace BoardsorterConfig.Views;

public partial class MainWindow : Window
{
    private readonly BoardsorterClient _client = new();
    private int _pingFailCount = 0;
    private DispatcherTimer? _pingTimer;

    public MainWindow()
    {
        InitializeComponent();
        // 侧边栏点击联动 Tab
        NavList.SelectionChanged += (s, e) =>
        {
            if (MainTabs is not null)
            {
                MainTabs.SelectedIndex = NavList.SelectedIndex;
            }
        };
        Opened += MainWindow_Opened;
    }

    private async void MainWindow_Opened(object? sender, EventArgs e)
    {
        // 启动时尝试连接 Go 端，失败则自动唤起主程序
        await TryStartBackendAsync();
        // 启动定时 ping：连续 3 次失败说明 Go 端退出了，GUI 也跟着退
        _pingTimer = new DispatcherTimer
        {
            Interval = TimeSpan.FromSeconds(2)
        };
        _pingTimer.Tick += PingTimer_Tick;
        _pingTimer.Start();
    }

    private void PingTimer_Tick(object? sender, EventArgs e)
    {
        _ = Task.Run(async () =>
        {
            bool ok = await _client.PingAsync();
            await Dispatcher.UIThread.InvokeAsync(() =>
            {
                if (ok)
                {
                    _pingFailCount = 0;
                }
                else
                {
                    _pingFailCount++;
                    if (_pingFailCount >= 3)
                    {
                        // Go 端已退出，关闭 GUI
                        _pingTimer?.Stop();
                        Close();
                    }
                }
            });
        });
    }

    private async Task TryStartBackendAsync()
    {
        if (await _client.PingAsync())
        {
            return;
        }
        // 后端没起，尝试唤起
        try
        {
            BoardsorterLauncher.Launch();
            // 等待 5 秒让 Go 端启动并写 ipc.json
            for (int i = 0; i < 10; i++)
            {
                await Task.Delay(500);
                _client.RefreshPort();
                if (await _client.PingAsync()) return;
            }
        }
        catch (Exception ex)
        {
            System.Diagnostics.Debug.WriteLine($"启动后端失败: {ex.Message}");
        }
    }
}
