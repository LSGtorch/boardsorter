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
    private Control[]? _pages;

    public MainWindow()
    {
        InitializeComponent();
        _pages = new Control[] { Page0, Page1, Page2, Page3, Page4, Page5, Page6 };
        NavList.SelectionChanged += (s, e) =>
        {
            SwitchPage(NavList.SelectedIndex);
        };
        Opened += MainWindow_Opened;
    }

    private void SwitchPage(int index)
    {
        if (_pages == null || index < 0 || index >= _pages.Length) return;
        for (int i = 0; i < _pages.Length; i++)
        {
            _pages[i].IsVisible = i == index;
        }
    }

    private async void MainWindow_Opened(object? sender, EventArgs e)
    {
        SwitchPage(NavList.SelectedIndex);
        await TryStartBackendAsync();
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
        try
        {
            BoardsorterLauncher.Launch();
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
