using System.Collections.Generic;

namespace BoardsorterConfig.Models;

public class ConfigModel
{
    public MonitorConfig Monitor { get; set; } = new();
    public AIConfig AI { get; set; } = new();
    public StartupConfig Startup { get; set; } = new();
    public ClassIslandConfig ClassIsland { get; set; } = new();
}

public class MonitorConfig
{
    public string WatchDir { get; set; } = "";
    public string ArchiveDir { get; set; } = "";
    public List<string> Subjects { get; set; } = new();
    public List<RuleItem> Rules { get; set; } = new();
}

public class RuleItem
{
    public string Pattern { get; set; } = "";
    public string Subject { get; set; } = "";
    public int Priority { get; set; }
}

public class AIConfig
{
    public string Endpoint { get; set; } = "";
    public string ApiKey { get; set; } = "";
    public string Model { get; set; } = "";
    public string ReasoningLevel { get; set; } = "medium";
    public string Prompt { get; set; } = "";
}

public class StartupConfig
{
    public bool AutoStart { get; set; }
    public bool StartMenuShortcut { get; set; }
    public int IpcPort { get; set; }
    public bool DarkMode { get; set; }
}

public class ClassIslandConfig
{
    public bool NotifyEnabled { get; set; }
    public string NotifyURL { get; set; } = "";
    public string NotifyTemplate { get; set; } = "";
}

public class ClassIslandNotification
{
    public string Time { get; set; } = "";
    public string FileName { get; set; } = "";
    public string Subject { get; set; } = "";
}

public class ClassIslandState
{
    public bool Connected { get; set; }
    public string CurrentClass { get; set; } = "";
    public string NextClass { get; set; } = "";
    public string ProfilePath { get; set; } = "";
    public string Error { get; set; } = "";
}

public class TermEntry
{
    public string Term { get; set; } = "";
    public string Subject { get; set; } = "";
    public int Count { get; set; }
}

public class FileMeta
{
    public string Path { get; set; } = "";
    public string Subject { get; set; } = "";
    public long Size { get; set; }
    public string ModifiedAt { get; set; } = "";
}

public class LogEntry
{
    public string Time { get; set; } = "";
    public string Level { get; set; } = "";
    public string Message { get; set; } = "";
}
