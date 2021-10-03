namespace Lachain.CommunicationHub.Net
{
    [SymbolName("StartHub")]
    public unsafe delegate int HubStart(byte* bootstrapAddress, int bootstrapAddressLen,
                                        byte* privKey, int privKeyLen, byte* networkName, int networkNameLen, 
                                        int version, int minPeerVersion, int chainId);

    [SymbolName("GetKey")]
    public unsafe delegate int HubGetKey(byte* buffer, int maxLength);

    [SymbolName("Init")]
    public unsafe delegate int HubInit(byte* signature, int signatureLength, int hubMetricsPort);

    [SymbolName("SendMessage")]
    public unsafe delegate int HubSendMessage(
        byte* pubKey, int pubKeyLen,
        byte* data, int dataLen
    );
    
    [SymbolName("GetMessages")]
    public unsafe delegate int HubGetMessages(byte* buffer, int maxLen);

    [SymbolName("StopHub")]
    public delegate bool HubStop();

    [SymbolName("LogLevel")]
    public unsafe delegate bool HubLogLevel(byte* str, int len);
    
    [SymbolName("StartProfiler")]
    public delegate int StartProfiler();

    [SymbolName("GenerateNewKey")]
    public unsafe delegate int HubGenerateNewKey(byte* str, int len);
}