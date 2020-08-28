using System;

namespace Lachain.CommunicationHub.Net
{
    public class Hub
    {
        internal readonly Lazy<StartHub> StartHub;
        internal readonly Lazy<StopHub> StopHub;

        
        const string Lib = "hub";

        private static readonly Lazy<string> LibPathLazy = new Lazy<string>(() => LibPathResolver.Resolve(Lib));
        private static readonly Lazy<IntPtr> LibPtr = new Lazy<IntPtr>(() => LoadLibNative.LoadLib(LibPathLazy.Value));

        internal static Hub Imports = new Hub();

        private Hub()
        {
            // load all delegates
            StartHub = LazyDelegate<StartHub>();
            StopHub = LazyDelegate<StopHub>();
        }

        Lazy<TDelegate> LazyDelegate<TDelegate>()
        {
            var symbol = SymbolNameCache<TDelegate>.SymbolName;
            return new Lazy<TDelegate>(
                () => LoadLibNative.GetDelegate<TDelegate>(LibPtr.Value, symbol),
                true
            );
        }

        public static void Start()
        {
            Imports.StartHub.Value();
        }
        
        public static void Stop()
        {
            Imports.StopHub.Value();
        }
    }
}