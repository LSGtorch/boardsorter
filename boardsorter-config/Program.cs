using System;
using System.Threading;
using Avalonia;
using Avalonia.Media;

namespace BoardsorterConfig;

internal static class Program
{
    private static Mutex? _mutex;

    [STAThread]
    public static void Main(string[] args)
    {
        // 防止重复启动 GUI
        _mutex = new Mutex(true, "BoardsorterConfig_UI_SingleInstance", out bool createdNew);
        if (!createdNew)
        {
            // 已有实例在运行，直接退出
            return;
        }

        try
        {
            BuildAvaloniaApp().StartWithClassicDesktopLifetime(args);
        }
        finally
        {
            _mutex?.ReleaseMutex();
            _mutex?.Dispose();
        }
    }

    public static AppBuilder BuildAvaloniaApp() =>
        AppBuilder.Configure<App>()
            .UsePlatformDetect()
            .WithInterFont()
            .With(new FontManagerOptions
            {
                DefaultFamilyName = "Microsoft YaHei UI,Microsoft YaHei,Segoe UI,Inter"
            })
            .LogToTrace();
}
