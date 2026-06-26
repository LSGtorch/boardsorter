using System;
using System.IO;
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
        // 全局异常捕获 - 写入日志文件
        AppDomain.CurrentDomain.UnhandledException += (sender, e) =>
        {
            var logPath = Path.Combine(AppContext.BaseDirectory, "crash.log");
            File.WriteAllText(logPath, $"[{DateTime.Now:yyyy-MM-dd HH:mm:ss}] Unhandled: {e.ExceptionObject}");
        };

        try
        {
            BuildAvaloniaApp().StartWithClassicDesktopLifetime(args);
        }
        catch (Exception ex)
        {
            var logPath = Path.Combine(AppContext.BaseDirectory, "crash.log");
            File.WriteAllText(logPath, $"[{DateTime.Now:yyyy-MM-dd HH:mm:ss}] {ex}");
            throw;
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