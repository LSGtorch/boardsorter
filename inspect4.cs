using System.Reflection;
using ClassIsland.Shared.IPC.Abstractions.Services;
using ClassIsland.Shared.IPC;

// Check the generated proxy type
var proxyTypeName = "ClassIsland.Shared.IPC.Abstractions.Services.__IPublicUriNavigationServiceIpcProxy";
var asm = typeof(IpcClient).Assembly;
var proxyType = asm.GetType(proxyTypeName);
if (proxyType != null)
{
    Console.WriteLine($"=== {proxyType.FullName} ===");
    Console.WriteLine($"  Base: {proxyType.BaseType?.FullName}");
    foreach (var attr in proxyType.GetCustomAttributes(false))
    {
        Console.WriteLine($"  Attr: {attr.GetType().Name}");
        foreach (var p in attr.GetType().GetProperties())
        {
            try { Console.WriteLine($"    {p.Name} = {p.GetValue(attr)}"); } catch {}
        }
    }
    foreach (var ctor in proxyType.GetConstructors())
    {
        var pars = string.Join(", ", ctor.GetParameters().Select(p => $"{p.ParameterType.Name} {p.Name}"));
        Console.WriteLine($"  Ctor({pars})");
    }
}

// Check IpcProxyJointAttribute
var attrType = asm.GetType("ClassIsland.Shared.IPC.AssemblyIpcProxyJointAttribute");
if (attrType != null)
{
    Console.WriteLine($"\n=== {attrType.FullName} ===");
    foreach (var p in attrType.GetProperties())
    {
        Console.WriteLine($"  {p.Name} : {p.PropertyType.Name}");
    }
    var attrs = asm.GetCustomAttributes(attrType, false);
    foreach (var a in attrs)
    {
        foreach (var p in attrType.GetProperties())
        {
            try { Console.WriteLine($"  {p.Name} = {p.GetValue(a)}"); } catch {}
        }
    }
}
