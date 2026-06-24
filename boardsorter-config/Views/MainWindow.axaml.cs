using System;
using System.Threading.Tasks;
using Avalonia.Controls;
using BoardsorterConfig.Services;

namespace BoardsorterConfig.Views;

public partial class MainWindow : Window
{
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
    }

    private async Task TryStartBackendAsync()
    {
        var client = new BoardsorterClient();
        if (await client.PingAsync()) return;

        // 后端没起，尝试唤起
        try
        {
            BoardsorterLauncher.Launch();
            // 等待 3 秒让 Go 端启动
            for (int i = 0; i < 6; i++)
            {
                await Task.Delay(500);
                if (await client.PingAsync()) return;
            }
        }
        catch (Exception ex)
        {
            // 唤起失败也不阻塞 UI
            System.Diagnostics.Debug.WriteLine($"启动后端失败: {ex.Message}");
        }
    }
}
