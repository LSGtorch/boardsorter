using System;
using Avalonia;
using Avalonia.Media;

namespace BoardsorterConfig;

internal static class Program
{
    [STAThread]
    public static void Main(string[] args) =>
        BuildAvaloniaApp().StartWithClassicDesktopLifetime(args);

    public static AppBuilder BuildAvaloniaApp() =>
        AppBuilder.Configure<App>()
            .UsePlatformDetect()
            .WithInterFont()
            // 优先用系统中文字体渲染（FluentAvalonia 默认 Inter 对中文支持差），
            // 找不到再回退到 Inter / Segoe UI。Win10/11 自带 Microsoft YaHei UI。
            .With(new FontManagerOptions
            {
                DefaultFamilyName = "Microsoft YaHei UI,Microsoft YaHei,Segoe UI,Inter"
            })
            .LogToTrace();
}
