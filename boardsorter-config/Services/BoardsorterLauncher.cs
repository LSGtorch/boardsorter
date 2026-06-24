using System;
using System.Diagnostics;
using System.IO;

namespace BoardsorterConfig.Services;

public static class BoardsorterLauncher
{
    /// <summary>
    /// 启动 boardsorter.exe 主程序
    /// 路径：当前 exe 所在目录的 boardsorter.exe
    /// </summary>
    public static void Launch()
    {
        // 当前 exe (boardsorter-ui.exe) 所在目录
        var uiDir = AppContext.BaseDirectory;
        // boardsorter.exe 平级
        var backendPath = Path.Combine(uiDir, "boardsorter.exe");

        if (!File.Exists(backendPath))
        {
            throw new FileNotFoundException($"找不到 boardsorter.exe: {backendPath}");
        }

        var psi = new ProcessStartInfo
        {
            FileName = backendPath,
            UseShellExecute = false,
            CreateNoWindow = true,
            WorkingDirectory = Path.GetDirectoryName(backendPath) ?? uiDir
        };
        Process.Start(psi);
    }
}
