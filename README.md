# boardsorter

高中教学文件自动分类归档工具。监控指定目录，自动将文件归类到对应学科文件夹，支持 AI 辅助分类和本地词条库精准匹配。

## 功能

- **目录监控**：监控指定文件夹，新文件出现后自动分类归档
- **AI 分类**：支持 OpenAI 兼容 API，轻量分析（仅文件名）和深度分析（提取文件内容）
- **词条库 BM25 匹配**：自动学习分类历史，下次同类文件秒级匹配，无需重复 AI 调用
- **文件元数据追踪**：UUID 追踪每个文件的归档记录，自动清理已删除文件记录
- **词条检索文件**：在词条库中点击词条，直接查看所有关联的已归档文件
- **图形配置界面**（Avalonia UI）：侧边栏导航，可视化配置
- **开机自启动 + 开始菜单快捷方式**：任务管理器启动项
- **深色模式 / 浅色模式**：实时切换
- **Windows Toast 通知**：文件分类后弹出系统通知，无需 ClassIsland 即可工作
- **系统托盘运行**：后台静默运行，托盘右键菜单打开 GUI 或退出

## 使用

1. 编辑 `config.ini` 配置监控目录和学科类目
2. 双击 `boardsorter.exe` 启动主程序（后台运行，系统托盘）
3. 双击 `BoardsorterConfig.exe` 启动图形配置界面，可查看状态、调整配置
4. 系统托盘右键菜单可打开 GUI 或退出程序

**需要 .NET 8 桌面运行时**（如使用 self-contained 版本则不需要）

## 配置说明

首次运行会自动创建 `data/` 数据目录。配置模板见 `config.example.ini`：

```ini
[路径配置]
归档根目录 = D:\Boards\Archive
科目文件夹列表 = 数学, 语文, 英语, 物理, 化学, 生物
无关文件夹 = D:\Boards\Archive\其他无关文件
无法确定类别文件夹 = D:\Boards\Archive\无法确定类别
日志文件夹 = D:\Boards\Archive\程序日志

[AI配置]
AI接口地址 = https://api.deepseek.com/v1/chat/completions
API密钥 = sk-xxx
模型名称 = deepseek-v4-flash
系统提示词 = 请根据以下文件名判断学科...
推理等级 = low
失败重试等待秒数 = 60
最大重试次数 = 1

[监控配置]
要监控的下载文件夹 = D:\Boards\Inbox
扫描间隔秒数 = 5

[规则配置]
下载源文件保留小时数 = 1
词条最大空闲天数 = 30
可读文档扩展名 = .docx,.pptx,.pdf,.txt
压缩包扩展名 = .zip,.rar,.7z

[IPC配置]
IPC端口 = 0
IPC绑定地址 = 127.0.0.1

[启动配置]
开机自启动 = false
开始菜单快捷方式 = false

[UI配置]
深色模式 = false

[ClassIsland通知]
启用通知 = false
API地址 = classisland://app/
通知模板 = {filename} → {subject}
```

### 配置说明

- `IPC端口` 设为 0 表示自动选择可用端口（59812-59820）
- `启用通知` 在文件分类后推送 Windows 原生 Toast 通知（无需 ClassIsland）
- 启动时自动追加缺失的配置项，保留用户原有配置

## 配置界面（图形页面说明）

| 页面 | 功能 |
|------|------|
| **监控配置** | 设置监控目录、归档目录、学科类目 |
| **AI 配置** | 配置 AI 接口地址、API 密钥、模型、推理等级、提示词 |
| **启动项** | 开机自启动、开始菜单快捷方式、IPC 端口设置 |
| **词条库** | 搜索词条、按科目筛选、点击词条查看关联文件、管理手动规则 |
| **文件元数据** | 查看所有已归档文件、按科目筛选 |
| **日志** | 实时查看程序运行日志 |
| **外观** | 深色/浅色模式切换 |
| **通知** | 启用/禁用文件分类 Toast 通知 |

## 通知机制

v1.3 开始使用 **Windows 原生 Toast 通知** 替代 ClassIsland IPC 导航：

- 文件被分类后，配置 GUI 通过 HTTP 轮询 Go 后端的通知队列
- 收到通知后通过 `Windows.UI.Notifications.ToastNotificationManager` 推送系统通知
- 不需要 ClassIsland 在后台运行，也不依赖其 IPC 接口
- 可在"通知"页面启用/禁用

对于仍需与 ClassIsland 联动的场景，GUI 会尝试检测 ClassIsland 运行状态并显示在界面上，但通知推送独立于 ClassIsland。

## 项目结构

```
boardsorter/
├── main.go                  # 入口，命令行解析
├── config.go                # 配置解析、INI 读写、自动追加缺失字段
├── ipc.go                   # HTTP IPC 服务（Go 作为服务端）
├── monitor.go               # 目录监控、文件分类
├── classifier.go            # 文件分类器（BM25 + AI 级联）
├── termdb.go                # 词条数据库（BM25 搜索、衰减）
├── metadata.go              # 文件元数据（UUID 追踪、删除清理）
├── ai.go                    # AI 分类（OpenAI 兼容 API）
├── classisland.go           # ClassIsland 通知队列
├── system_windows.go        # Windows 系统集成（自启动、快捷方式、控制台检测）
├── system_other.go          # 非 Windows 系统桩
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
│       ├── BoardsorterLauncher.cs          # 后端启动器
│       └── ClassIslandIpcBridge.cs         # Windows Toast 通知桥接
├── config.example.ini       # 配置文件模板
└── README.md
```

## 编译

### Go 主程序

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-H windowsgui -s -w" -o boardsorter.exe .
```

`-H windowsgui` 隐藏控制台窗口，`-s -w` 去除调试符号减小体积。

### Avalonia GUI

```bash
cd boardsorter-config
dotnet publish -c Release -r win-x64 --self-contained true -p:PublishSingleFile=true
```

需要 .NET 8 SDK。如从 Linux 交叉编译，csproj 中需设置 `<EnableWindowsTargeting>true</EnableWindowsTargeting>`。

## API

GUI 通过 HTTP IPC 与 Go 主程序通信，端口写在 `data/ipc.json`。

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/ping | 心跳检测 |
| GET | /api/status | 获取运行状态 |
| GET | /api/config | 获取配置 |
| POST | /api/config | 保存配置 |
| GET | /api/terms | 获取词条库（支持 ?keyword=xxx&subject=xxx） |
| GET | /api/files | 获取文件元数据（支持 ?subject=xxx&term=xxx） |
| GET | /api/files/{uuid} | 获取单个文件详情 |
| GET | /api/logs | 获取日志 |
| POST | /api/scan | 触发手动扫描 |
| GET | /api/stats | 获取统计信息 |
| POST | /api/decay | 触发词条衰减 |
| POST | /api/stop | 优雅停止服务 |
| POST | /api/system/startmenu | 管理开始菜单快捷方式 |
| GET | /api/classisland/notifications | 获取待发送通知队列 |

## 技术栈

- Go 1.22+（标准库为主，无第三方依赖的 Windows API 调用）
- .NET 8 + Avalonia UI 11 + FluentAvalonia
- CommunityToolkit.Mvvm（MVVM 框架）
- HTTP IPC（Go net/http + C# HttpClient）
- Windows.UI.Notifications（原生 Toast 通知）