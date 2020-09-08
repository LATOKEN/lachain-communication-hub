// ReSharper disable InconsistentNaming

namespace Lachain.CommunicationHub.Net
{
    [SymbolName(nameof(StartHub))]
    public unsafe delegate int StartHub(byte* str, int len);

    [SymbolName(nameof(StopHub))]
    public delegate bool StopHub();

    [SymbolName(nameof(LogLevel))]
    public unsafe delegate bool LogLevel(byte* str, int len);
}
