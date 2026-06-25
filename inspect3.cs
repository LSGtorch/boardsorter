using System.Reflection;
using dotnetCampus.Ipc.CompilerServices.GeneratedProxies;

var factoryType = typeof(GeneratedIpcFactory);
Console.WriteLine($"=== {factoryType.FullName} ===");
foreach (var m in factoryType.GetMethods(BindingFlags.Public | BindingFlags.Static))
{
    var pars = string.Join(", ", m.GetParameters().Select(p => $"{p.ParameterType.Name} {p.Name}"));
    Console.WriteLine($"  {m.ReturnType.Name} {m.Name}<{string.Join(",", m.GetGenericArguments().Select(t => t.Name))}>({pars})");
}
