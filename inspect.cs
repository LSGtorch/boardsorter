using System.Reflection;
using ClassIsland.Shared.IPC;
using ClassIsland.Shared.IPC.Abstractions.Services;

// Inspect IpcClient
var clientType = typeof(IpcClient);
Console.WriteLine($"=== {clientType.FullName} ===");

foreach (var m in clientType.GetMethods(BindingFlags.Public | BindingFlags.Instance | BindingFlags.Static | BindingFlags.DeclaredOnly))
{
    var pars = string.Join(", ", m.GetParameters().Select(p => $"{p.ParameterType.Name} {p.Name}"));
    Console.WriteLine($"  {m.ReturnType.Name} {m.Name}({pars})");
}
foreach (var p in clientType.GetProperties(BindingFlags.Public | BindingFlags.Instance))
{
    Console.WriteLine($"  Prop: {p.PropertyType.FullName} {p.Name}");
}

var providerType = clientType.GetProperty("Provider")?.PropertyType;
if (providerType != null)
{
    Console.WriteLine($"\n=== {providerType.FullName} ===");
    foreach (var m in providerType.GetMethods(BindingFlags.Public | BindingFlags.Instance | BindingFlags.Static | BindingFlags.DeclaredOnly))
    {
        var pars = string.Join(", ", m.GetParameters().Select(p => $"{p.ParameterType.Name} {p.Name}"));
        Console.WriteLine($"  {m.ReturnType.Name} {m.Name}({pars})");
    }
}

var uriNavType = typeof(IPublicUriNavigationService);
Console.WriteLine($"\n=== {uriNavType.FullName} ===");
foreach (var m in uriNavType.GetMethods(BindingFlags.Public | BindingFlags.Instance | BindingFlags.DeclaredOnly))
{
    var pars = string.Join(", ", m.GetParameters().Select(p => $"{p.ParameterType.Name} {p.Name}"));
    Console.WriteLine($"  {m.ReturnType.Name} {m.Name}({pars})");
}
