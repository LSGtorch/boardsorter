using System.Reflection;
using dotnetCampus.Ipc.CompilerServices.GeneratedProxies;

// Check GeneratedIpcProxy<T>
var proxyBaseType = typeof(GeneratedIpcProxy<>);
Console.WriteLine($"=== {proxyBaseType.FullName} ===");
foreach (var ctor in proxyBaseType.GetConstructors(BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Instance))
{
    var pars = string.Join(", ", ctor.GetParameters().Select(p => $"{p.ParameterType.Name} {p.Name}"));
    Console.WriteLine($"  Ctor({pars})");
}
Console.WriteLine("---Properties---");
foreach (var p in proxyBaseType.GetProperties(BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Instance))
{
    Console.WriteLine($"  {p.PropertyType.Name} {p.Name}");
}

// Check GeneratedIpcFactory.CreateIpcProxy with default ipcObjectId
// What happens when we pass empty string?
Console.WriteLine("\n---Testing CreateIpcProxy with empty string---");
var factoryType = typeof(GeneratedIpcFactory);
foreach (var m in factoryType.GetMethods(BindingFlags.Public | BindingFlags.Static))
{
    if (m.Name == "CreateIpcProxy" && m.IsGenericMethod)
    {
        var pars = m.GetParameters();
        Console.WriteLine($"  Method: {m.Name}");
        Console.WriteLine($"  Generic args: {m.GetGenericArguments().Length}");
        foreach (var p in pars)
        {
            Console.WriteLine($"    {p.ParameterType.Name} {p.Name} (default: {p.HasDefaultValue})");
            if (p.HasDefaultValue)
                Console.WriteLine($"      default = {p.DefaultValue ?? "null"}");
        }
    }
}
