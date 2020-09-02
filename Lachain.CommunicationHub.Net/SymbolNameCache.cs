using System.Reflection;

namespace Lachain.CommunicationHub.Net
{
    // Taken from: https://github.com/MeadowSuite/Secp256k1.Net/blob/master/Secp256k1.Net/SymbolNameCache.cs
    static class SymbolNameCache<TDelegate>
    {
        public static readonly string SymbolName;

        static SymbolNameCache()
        {
            SymbolName = typeof(TDelegate).GetCustomAttribute<SymbolNameAttribute>().Name;
        }
    }
}