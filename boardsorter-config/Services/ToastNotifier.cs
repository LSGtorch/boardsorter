using System;
using Windows.Data.Xml.Dom;
using Windows.UI.Notifications;

namespace BoardsorterConfig.Services;

/// <summary>
/// 通过 Windows 原生 Toast 通知发送文件分类通知。
/// </summary>
public class ToastNotifier : IDisposable
{
    private string _lastError = "";
    public string LastError => _lastError;

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
    }
}