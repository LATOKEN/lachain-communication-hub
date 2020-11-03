using System;
using System.Linq;
using System.Net;
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
        }

        Lazy<TDelegate> LazyDelegate<TDelegate>()
        {
            var symbol = SymbolNameCache<TDelegate>.SymbolName;
            return new Lazy<TDelegate>(
                () => LoadLibNative.GetDelegate<TDelegate>(LibPtr.Value, symbol),
                true
            );
        }

        public static void Start(string grpcAddress, string bootstrapAddress)
        {
            unsafe
            {
                var grpcAddressBytes = Encoding.UTF8.GetBytes(grpcAddress);
                var bootstrapAddressBytes = Encoding.UTF8.GetBytes(bootstrapAddress);
                fixed (byte* grpcAddressPtr = grpcAddressBytes)
                fixed (byte* bootstrapAddressPtr = bootstrapAddressBytes)
                {
                    Imports.StartHub.Value(
                        grpcAddressPtr, grpcAddressBytes.Length,
                        bootstrapAddressPtr, bootstrapAddressBytes.Length
                    );
                }
            }
        }

        public static bool Init(byte[] signature)
        {
            unsafe
            {
                fixed (byte* signaturePtr = signature)
                {
                    return Imports.HubInit.Value(signaturePtr, signature.Length) == 1;
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
        const int maxBufferSize = 32 * 1024 * 1024; // 32MiB
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
                        var len = (int) ParseLittleEndianInt32(_buffer.AsSpan().Slice(ptr, 4));
                        ptr += 4;
                        ret[i] = _buffer.AsSpan().Slice(ptr, len).ToArray();
                        ptr += len;
                    }

                    return ret;
                }

                if (_buffer.Length * 2 > maxBufferSize)
                {
                    throw new Exception("Cannot read message from hub: max buffer size is too small");
                }
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
    }
}