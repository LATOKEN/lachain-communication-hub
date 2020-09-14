// ReSharper disable InconsistentNaming

namespace Lachain.CommunicationHub.Net
{
    [SymbolName(nameof(StartHub))]
    public unsafe delegate int StartHub(
        byte* grpcAddress, int grpcAddressLen,
        byte* bootstrapAddress, int bootstrapAddressLen
    );

    [SymbolName(nameof(StopHub))]
    public delegate bool StopHub();

    [SymbolName(nameof(LogLevel))]
    public unsafe delegate bool LogLevel(byte* str, int len);
}
