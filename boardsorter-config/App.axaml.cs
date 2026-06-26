using Avalonia;
using Avalonia.Controls;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.Markup.Xaml;
using BoardsorterConfig.ViewModels;
using BoardsorterConfig.Views;
using System;
using System.IO;

namespace BoardsorterConfig;

public partial class App : Application
{
    private MainWindow? _mainWindow;
    private TrayIcon? _trayIcon;
    private EventHandler<WindowClosingEventArgs>? _mainWindowClosingHandler;

    public override void Initialize()
    {
        AvaloniaXamlLoader.Load(this);
    }

    public override void OnFrameworkInitializationCompleted()
    {
        if (ApplicationLifetime is IClassicDesktopStyleApplicationLifetime desktop)
        {
            _mainWindow = new MainWindow();
            desktop.MainWindow = _mainWindow;

            SetupTrayIcon();

            // 点击关闭按钮时隐藏到托盘而非退出
        _mainWindowClosingHandler = (sender, args) =>
        {
            args.Cancel = true;
            _mainWindow!.Hide();
        };
        _mainWindow.Closing += _mainWindowClosingHandler;

            // 点击托盘图标显示窗口
            _trayIcon!.Clicked += (sender, args) =>
            {
                if (_mainWindow == null) return;
                if (_mainWindow.IsVisible)
                {
                    _mainWindow.Hide();
                }
                else
                {
                    _mainWindow.Show();
                    _mainWindow.Activate();
                }
            };
        }

        base.OnFrameworkInitializationCompleted();
    }

    private void SetupTrayIcon()
    {
        WindowIcon? icon = null;
        try
        {
            var iconPath = System.IO.Path.Combine(AppContext.BaseDirectory, "Assets", "appicon.ico");
            if (System.IO.File.Exists(iconPath))
                icon = new WindowIcon(iconPath);
        }
        catch { }

        _trayIcon = new TrayIcon
        {
            ToolTipText = "Boardsorter",
            Icon = icon
        };

        var showItem = new NativeMenuItem("显示主界面");
        showItem.Click += (_, _) =>
        {
            if (_mainWindow == null) return;
            _mainWindow.Show();
            _mainWindow.Activate();
        };

        var exitItem = new NativeMenuItem("退出");
        exitItem.Click += (_, _) =>
        {
            if (_trayIcon != null)
            {
                _trayIcon.Dispose();
                _trayIcon = null;
            }
            if (_mainWindow != null)
            {
                _mainWindow.Closing -= _mainWindowClosingHandler;
            }
            if (ApplicationLifetime is IClassicDesktopStyleApplicationLifetime desk)
            {
                desk.Shutdown();
            }
        };

        var menu = new NativeMenu();
        menu.Items.Add(showItem);
        menu.Items.Add(new NativeMenuItemSeparator());
        menu.Items.Add(exitItem);

        _trayIcon.Menu = menu;
    }

    /// <summary>
    /// 托盘图标（供 ViewModel 等其他代码访问）
    /// </summary>
    public TrayIcon? TrayIcon => _trayIcon;
}