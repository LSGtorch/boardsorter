using System;
using System.Threading.Tasks;
using ClassIsland.Shared.IPC;
using ClassIsland.Shared.IPC.Abstractions.Services;
using dotnetCampus.Ipc.CompilerServices.GeneratedProxies;

namespace BoardsorterConfig.Services;

/// <summary>
/// 通过 ClassIsland.Shared.IPC 命名管道连接到 ClassIsland，
/// 使用 IPublicUriNavigationService 在 ClassIsland 中触发导航/通知
/// </summary>
public class ClassIslandIpcBridge : IDisposable
{
    private IpcClient? _client;
    private IPublicUriNavigationService? _uriNav;
    private bool _connected;
    private string _lastError = "";

    public bool Connected => _connected;
    public string LastError => _lastError;

    public async Task ConnectAsync()
    {
        try
        {
            _client = new IpcClient();
            await _client.Connect();
            _uriNav = GeneratedIpcFactory.CreateIpcProxy<IPublicUriNavigationService>(
                _client.Provider, _client.PeerProxy!);
            _connected = true;
            _lastError = "";
        }
        catch (Exception ex)
        {
            _connected = false;
            _lastError = ex.Message;
        }
    }

    /// <summary>
    /// 发送文件分类通知到 ClassIsland。
    /// 使用 IPublicUriNavigationService 导航到配置的 ClassIsland URI，
    /// 通知内容通过 URI query 参数传递。
    /// </summary>
    public void SendNotification(string fileName, string subject, string notifyUrl, string template)
    {
        if (!_connected || _uriNav == null)
            return;
        try
        {
            var message = template
                .Replace("{filename}", fileName)
                .Replace("{subject}", subject);

            var encodedMessage = Uri.EscapeDataString(message);
            var separator = notifyUrl.Contains('?') ? "&" : "?";
            var fullUri = new Uri($"{notifyUrl}{separator}boardsorter_msg={encodedMessage}");

            _uriNav.Navigate(fullUri);
        }
        catch (Exception ex)
        {
            _lastError = ex.Message;
            _connected = false;
        }
    }

    public void Dispose()
    {
        _client?.Provider.Dispose();
        _connected = false;
    }
}