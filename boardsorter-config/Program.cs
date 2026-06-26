using System;
using System.IO;

using Avalonia;
using Avalonia.Media;

namespace BoardsorterConfig;

internal static class Program
{
    [STAThread]
    public static void Main(string[] args)
    {
        // 全局异常捕获 - 写入日志文件
        AppDomain.CurrentDomain.UnhandledException += (sender, e) =>
        {
            try
            {
                var logPath = Path.Combine(AppContext.BaseDirectory, "crash.log");
                File.WriteAllText(logPath, $"[{DateTime.Now:yyyy-MM-dd HH:mm:ss}] Unhandled: {e.ExceptionObject}");
            }
            catch { }
        };

        try
        {
            // 启动时写入启动日志（帮助排查打不开的问题）
            var launchLog = Path.Combine(AppContext.BaseDirectory, "launch.log");
            File.WriteAllText(launchLog, $"[{DateTime.Now:yyyy-MM-dd HH:mm:ss}] Starting BoardsorterConfig...\n");
            File.AppendAllText(launchLog, $"[{DateTime.Now:yyyy-MM-dd HH:mm:ss}] IsSingleFile: {string.IsNullOrEmpty(Environment.GetCommandLineArgs()[0])}\n");
            File.AppendAllText(launchLog, $"[{DateTime.Now:yyyy-MM-dd HH:mm:ss}] BaseDir: {AppContext.BaseDirectory}\n");

            BuildAvaloniaApp().StartWithClassicDesktopLifetime(args);

            File.AppendAllText(launchLog, $"[{DateTime.Now:yyyy-MM-dd HH:mm:ss}] Exited normally\n");
        }
        catch (Exception ex)
        {
            try
            {
                var logPath = Path.Combine(AppContext.BaseDirectory, "crash.log");
                File.WriteAllText(logPath, $"[{DateTime.Now:yyyy-MM-dd HH:mm:ss}] {ex}");
            }
            catch { }
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