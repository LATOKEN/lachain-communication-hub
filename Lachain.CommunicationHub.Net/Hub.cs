using System;
using System.Linq;
using System.Text;

namespace Lachain.CommunicationHub.Net
{
    public class Hub
    {
        internal readonly Lazy<HubStart> StartHub;
        internal readonly Lazy<HubStop> StopHub;
        internal readonly Lazy<HubLogLevel> LogLevel;
        internal readonly Lazy<HubSendMessage> SendMessage;
        internal readonly Lazy<HubGetMessages> GetMessages;
        internal readonly Lazy<HubInit> HubInit;
        internal readonly Lazy<HubGetKey> HubGetKey;
        internal readonly Lazy<StartProfiler> StartProfiler;
        internal readonly Lazy<HubGenerateNewKey> GenerateNewKeyHub;


        const string Lib = "hub";

        private static readonly Lazy<string> LibPathLazy = new Lazy<string>(() => LibPathResolver.Resolve(Lib));
        private static readonly Lazy<IntPtr> LibPtr = new Lazy<IntPtr>(() => LoadLibNative.LoadLib(LibPathLazy.Value));

        internal static Hub Imports = new Hub();

        private Hub()
        {
            // load all delegates
            StartHub = LazyDelegate<HubStart>();
            StopHub = LazyDelegate<HubStop>();
            LogLevel = LazyDelegate<HubLogLevel>();
            SendMessage = LazyDelegate<HubSendMessage>();
            GetMessages = LazyDelegate<HubGetMessages>();
            HubInit = LazyDelegate<HubInit>();
            HubGetKey = LazyDelegate<HubGetKey>();
            StartProfiler = LazyDelegate<StartProfiler>();
            GenerateNewKeyHub = LazyDelegate<HubGenerateNewKey>();
        }

        Lazy<TDelegate> LazyDelegate<TDelegate>()
        {
            var symbol = SymbolNameCache<TDelegate>.SymbolName;
            return new Lazy<TDelegate>(
                () => LoadLibNative.GetDelegate<TDelegate>(LibPtr.Value, symbol),
                true
            );
        }

        public static void Start(string bootstrapAddress,  byte[] privKey, string networkName, int version, int minPeerVersion, int chainId)
        {
            unsafe
            {
                var bootstrapAddressBytes = Encoding.UTF8.GetBytes(bootstrapAddress);
                var networkNameBytes = Encoding.UTF8.GetBytes(networkName);
                fixed (byte* bootstrapAddressPtr = bootstrapAddressBytes)
                fixed (byte* privKeyPtr = privKey)
                fixed (byte* networkNamePtr = networkNameBytes)
                {
                    Imports.StartHub.Value(
                        bootstrapAddressPtr, bootstrapAddressBytes.Length,
                        privKeyPtr, privKey.Length, networkNamePtr, networkNameBytes.Length, 
                        version, minPeerVersion,  chainId
                    );
                }
            }
        }

        public static bool Init(byte[] signature, int hubMetricsPort)
        {
            unsafe
            {
                fixed (byte* signaturePtr = signature)
                {
                    return Imports.HubInit.Value(signaturePtr, signature.Length, hubMetricsPort) == 1;
                }
            }
        }

        public static byte[] GetKey()
        {
            const int maxKeyLen = 100;
            var key = new byte[maxKeyLen];
            unsafe
            {
                fixed (byte* keyPtr = key)
                {
                    var result = Imports.HubGetKey.Value(keyPtr, maxKeyLen);
                    return key.Take(result).ToArray();
                }
            }
        }
        
        const int initialBufferSize = 1024 * 1024; // 1MiB
        // const int maxBufferSize = 32 * 1024 * 1024; // 32MiB
        private static byte[] _buffer = new byte[initialBufferSize];

        private static uint ParseLittleEndianInt32(Span<byte> span)
        {
            return BitConverter.ToUInt32(BitConverter.IsLittleEndian ? span : span.ToArray().Reverse().ToArray());
        }

        public static byte[][] Get()
        {
            while (true)
            {
                int result;
                unsafe
                {
                    fixed (byte* bufferPtr = _buffer)
                    {
                        result = Imports.GetMessages.Value(bufferPtr, _buffer.Length);
                    }
                }

                if (result < 0) return Array.Empty<byte[]>();
                if (result > 0)
                {
                    var ret = new byte[result][];
                    var ptr = 0;
                    for (var i = 0; i < result; ++i)
                    {
                        if (ptr + 4 > _buffer.Length) throw new Exception("Buffer overflow");
                        var len = (int) ParseLittleEndianInt32(_buffer.AsSpan().Slice(ptr, 4));
                        ptr += 4;
                        if (ptr + len > _buffer.Length) throw new Exception("Buffer overflow");
                        ret[i] = _buffer.AsSpan().Slice(ptr, len).ToArray();
                        ptr += len;
                    }

                    return ret;
                }

                // if (_buffer.Length * 2 > maxBufferSize)
                // {
                //     throw new Exception("Cannot read message from hub: max buffer size is too small");
                // }
                _buffer = new byte[_buffer.Length * 2];
            }
        }

        public static void Send(byte[] publicKey, byte[] data)
        {
            unsafe
            {
                fixed (byte* publicKeyPtr = publicKey)
                fixed (byte* dataPtr = data)
                {
                    Imports.SendMessage.Value(publicKeyPtr, publicKey.Length, dataPtr, data.Length);
                }
            }
        }

        public static void Stop()
        {
            Imports.StopHub.Value();
        }

        public static int GetProfilerPort()
        {
            return Imports.StartProfiler.Value();
        }

        public static void SetLogLevel(string s)
        {
            unsafe
            {
                var bytes = Encoding.UTF8.GetBytes(s);
                fixed (byte* ptr = bytes)
                {
                    Imports.LogLevel.Value(ptr, bytes.Length);
                }
            }
        }

        public static string GenerateNewHubKey()
        {
            const int maxBuffer = 2000;
            byte[] privHex = new byte[maxBuffer];
            int len = maxBuffer;
            unsafe
            {
                fixed (byte* ptr = privHex)
                {
                    len = Imports.GenerateNewKeyHub.Value(ptr, len);
                }
            }
            return Encoding.UTF8.GetString(privHex.Take(len).ToArray());
        }
    }
}