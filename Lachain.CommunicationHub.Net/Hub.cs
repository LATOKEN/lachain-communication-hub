using System;
using System.Text;

namespace Lachain.CommunicationHub.Net
{
    public class Hub
    {
        internal readonly Lazy<StartHub> StartHub;
        internal readonly Lazy<StartHubOnPort> StartHubOnPort;
        internal readonly Lazy<StopHub> StopHub;
        internal readonly Lazy<LogLevel> LogLevel;

        
        const string Lib = "hub";

        private static readonly Lazy<string> LibPathLazy = new Lazy<string>(() => LibPathResolver.Resolve(Lib));
        private static readonly Lazy<IntPtr> LibPtr = new Lazy<IntPtr>(() => LoadLibNative.LoadLib(LibPathLazy.Value));

        internal static Hub Imports = new Hub();

        private Hub()
        {
            // load all delegates
            StartHub = LazyDelegate<StartHub>();
            StartHubOnPort = LazyDelegate<StartHubOnPort>();
            StopHub = LazyDelegate<StopHub>();
            LogLevel = LazyDelegate<LogLevel>();
        }

        Lazy<TDelegate> LazyDelegate<TDelegate>()
        {
            var symbol = SymbolNameCache<TDelegate>.SymbolName;
            return new Lazy<TDelegate>(
                () => LoadLibNative.GetDelegate<TDelegate>(LibPtr.Value, symbol),
                true
            );
        }

        public static void StartOnPort(string port)
        {
            unsafe
            {
                var bytes = Encoding.UTF8.GetBytes(port);
                fixed (byte* ptr = bytes)
                {
                    Imports.StartHubOnPort.Value(ptr, bytes.Length);
                }                
            }
        }
        
        public static void Start()
        {
            Imports.StartHub.Value();
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
