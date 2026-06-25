using System;
using System.Threading.Tasks;
using Windows.Data.Xml.Dom;
using Windows.UI.Notifications;

namespace BoardsorterConfig.Services;

/// <summary>
/// 通过 Windows 原生 Toast 通知发送文件分类通知。
/// 直接在 Windows 通知中心显示，不依赖 ClassIsland IPC。
/// </summary>
public class ClassIslandIpcBridge : IDisposable
{
    private bool _connected;
    private string _lastError = "";

    public bool Connected => _connected;
    public string LastError => _lastError;

    /// <summary>
    /// 检查 ClassIsland 是否在运行（仅用于状态显示）
    /// </summary>
    public async Task ConnectAsync()
    {
        try
        {
            var client = new ClassIsland.Shared.IPC.IpcClient();
            await client.Connect();
            _connected = true;
            _lastError = "";
            client.Provider.Dispose();
        }
        catch (Exception ex)
        {
            _connected = false;
            _lastError = ex.Message;
        }
    }

    /// <summary>
    /// 发送 Windows Toast 通知
    /// </summary>
    public void SendNotification(string fileName, string subject)
    {
        try
        {
            var template = ToastNotificationManager.GetTemplateContent(ToastTemplateType.ToastText02);
            var elements = template.GetElementsByTagName("text");
            elements[0].AppendChild(template.CreateTextNode("Boardsorter - 文件分类完成"));
            elements[1].AppendChild(template.CreateTextNode($"{fileName} → {subject}"));

            var toast = new ToastNotification(template);
            ToastNotificationManager.CreateToastNotifier().Show(toast);
            _lastError = "";
        }
        catch (Exception ex)
        {
            _lastError = ex.Message;
        }
    }

    public void Dispose()
    {
        _connected = false;
    }
}