// ReSharper disable InconsistentNaming

namespace Lachain.CommunicationHub.Net
{
    [SymbolName(nameof(StartHub))]
    public delegate int StartHub();

    [SymbolName(nameof(StopHub))]
    public delegate bool StopHub();
}