# boardsorter

题目文件自动分类归档工具。监控指定目录，根据文件名规则自动将文件归类到对应学科文件夹。

## 功能

- 监控目录，自动分类归档文件到学科文件夹
- 基于规则的分类（文件名模式匹配）和 AI 辅助分类（可选，需要配置 API）
- 词条库：记录分类历史，支持 BM25 搜索
- 文件元数据管理（UUID 追踪）
- 系统托盘图标，后台静默运行
- 图形配置界面（Avalonia UI），侧边栏导航
- 开机自启动（任务管理器启动项）+ 开始菜单快捷方式
- 深色模式 / 浅色模式切换
- ClassIsland 联动：读取课表，显示当前课程信息

## 使用

1. 双击 `BoardsorterConfig.exe` 启动 GUI 配置界面，自动唤起主程序
2. 或双击 `boardsorter.exe` 直接启动主程序（后台运行，系统托盘）
3. 系统托盘右键菜单可打开 GUI 或退出程序
4. 首次使用请编辑 `config.ini` 配置监控目录和学科类目

**需要 .NET 8 桌面运行时**

## 配置说明

### config.ini

```ini
[路径配置]
监控目录 = D:\Boards\Inbox
归档目录 = D:\Boards\Archive
科目文件夹列表 = 语文, 数学, 英语, 物理, 化学, 生物, 历史, 地理, 政治

[AI配置]
端点 = https://api.openai.com/v1
API密钥 = sk-xxx
模型 = gpt-4o-mini
推理等级 = low
提示词 = 请根据文件名判断属于哪个学科...

[程序配置]
开机自启动 = false
开始菜单快捷方式 = false
IPC端口 = 0

[规则配置]
1 = 模式:数学*, 学科:数学, 优先级:10
2 = 模式:物理*, 学科:物理, 优先级:10

[UI配置]
深色模式 = false

[ClassIsland配置]
启用联动 = false
配置文件路径 =
```

- `IPC端口` 设为 0 表示自动选择可用端口（59812-59820）
- 启动时自动追加缺失的配置项，保留用户原有配置
- `[规则配置]` 格式：`序号 = 模式:xxx, 学科:xxx, 优先级:xxx`

## 项目结构

```
boardsorter/
├── main.go                  # 入口，命令行解析
├── config.go                # 配置解析、INI 读写、自动追加缺失字段
├── ipc.go                   # HTTP IPC 服务，供 GUI 调用
├── monitor.go               # 目录监控、文件分类
├── terms.go                 # 词条库管理、BM25 搜索
├── files.go                 # 文件元数据（UUID 追踪）
├── ai.go                    # AI 分类（OpenAI 兼容 API）
├── classisland.go           # ClassIsland 课表读取
├── system_windows.go        # Windows 系统集成（自启动、快捷方式、控制台检测）
├── system_other.go          # 非 Windows 系统桩
├── helpers.go               # 工具函数
├── logger.go                # 日志系统
├── go.mod / go.sum
├── boardsorter-config/      # Avalonia 图形配置界面
│   ├── Program.cs
│   ├── App.axaml / App.axaml.cs
│   ├── Views/
│   │   └── MainWindow.axaml / .axaml.cs   # 主窗口
│   ├── ViewModels/
│   │   └── MainWindowViewModel.cs         # MVVM ViewModel
│   ├── Models/
│   │   └── ConfigModel.cs                 # 数据模型
│   └── Services/
│       ├── BoardsorterClient.cs            # HTTP IPC 客户端
│       └── BoardsorterLauncher.cs          # 后端启动器
└── config.example.ini       # 配置文件模板
```

## 编译

### Go 主程序

```bash
GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui -s -w" -o boardsorter.exe .
```

`-H windowsgui` 隐藏控制台窗口，`-s -w` 去除调试符号减小体积。

### Avalonia GUI

```bash
cd boardsorter-config
dotnet publish -c Release -r win-x64 --self-contained false /p:PublishSingleFile=true
```

## API

GUI 通过 HTTP IPC 与 Go 主程序通信，端口写在 `data/ipc.json`。

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/ping | 心跳检测 |
| GET | /api/config | 获取配置 |
| POST | /api/config | 保存配置 |
| GET | /api/terms | 获取词条库 |
| GET | /api/files | 获取文件元数据 |
| GET | /api/logs | 获取日志 |
| POST | /api/system/startmenu | 管理开始菜单快捷方式 |
| GET | /api/classisland | 获取 ClassIsland 课表状态 |

## 技术栈

- Go 1.22+（标准库为主，无第三方依赖的 Windows API 调用）
- .NET 8 + Avalonia UI 11 + FluentAvalonia
- CommunityToolkit.Mvvm（MVVM 框架）
- HTTP IPC（Go net/http + C# HttpClient）