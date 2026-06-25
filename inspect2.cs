using System.Reflection;
using dotnetCampus.Ipc.Pipes;

var providerType = typeof(IpcProvider);
Console.WriteLine($"=== {providerType.FullName} ===");
foreach (var m in providerType.GetMethods(BindingFlags.Public | BindingFlags.Instance | BindingFlags.Static | BindingFlags.DeclaredOnly))
{
    var pars = string.Join(", ", m.GetParameters().Select(p => $"{p.ParameterType.Name} {p.Name}"));
    Console.WriteLine($"  {m.ReturnType.Name} {m.Name}({pars})");
}

// Check for extension methods
var asm = Assembly.GetAssembly(typeof(IpcProvider));
if (asm != null)
{
    Console.WriteLine($"\n=== Extension methods in {asm.GetName().Name} ===");
    foreach (var t in asm.GetTypes())
    {
        foreach (var m in t.GetMethods(BindingFlags.Public | BindingFlags.Static))
        {
            if (m.IsDefined(typeof(System.Runtime.CompilerServices.ExtensionAttribute), false))
            {
                var extendType = m.GetParameters().FirstOrDefault()?.ParameterType;
                if (extendType != null && (extendType.Name.Contains("Ipc") || extendType.Name.Contains("Peer")))
                {
                    var pars = string.Join(", ", m.GetParameters().Select(p => $"{p.ParameterType.Name} {p.Name}"));
                    Console.WriteLine($"  {t.Name}.{m.Name}({pars})");
                }
            }
        }
    }
}
