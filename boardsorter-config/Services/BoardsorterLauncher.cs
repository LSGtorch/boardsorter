using System;
using System.Diagnostics;
using System.IO;
using System.Linq;

namespace BoardsorterConfig.Services;

public static class BoardsorterLauncher
{
    private static readonly string _exeDir = Path.GetDirectoryName(Environment.ProcessPath)
        ?? AppContext.BaseDirectory;

    /// <summary>
    /// 启动 boardsorter.exe 主程序（如果未运行）
    /// </summary>
    public static void Launch()
    {
        var backendPath = Path.Combine(_exeDir, "boardsorter.exe");

        if (!File.Exists(backendPath))
        {
            throw new FileNotFoundException($"找不到 boardsorter.exe: {backendPath}");
        }

        // 检查 boardsorter 是否已经在运行
        if (IsBackendRunning())
        {
            return;
        }

        var psi = new ProcessStartInfo
        {
            FileName = backendPath,
            UseShellExecute = false,
            CreateNoWindow = true,
            WindowStyle = ProcessWindowStyle.Hidden,
            WorkingDirectory = _exeDir
        };
        Process.Start(psi);
    }

    /// <summary>
    /// 检查 boardsorter 后端是否已在运行
    /// </summary>
    public static bool IsBackendRunning()
    {
        try
        {
            var procs = Process.GetProcessesByName("boardsorter");
            // 排除自身（BoardsorterConfig.exe）
            return procs.Any(p =>
            {
                try { return p.Id != Environment.ProcessId; }
                catch { return false; }
            });
        }
        catch
        {
            return false;
        }
    }
}
