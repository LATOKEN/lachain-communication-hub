using System;
using System.Runtime.InteropServices;

namespace Lachain.CommunicationHub.Net.DynamicLinking
{
    // ReSharper disable once InconsistentNaming
    // Taken from: https://github.com/MeadowSuite/Secp256k1.Net/blob/master/Secp256k1.Net/DynamicLinking/DynamicLinkingMacOS.cs
    static class DynamicLinkingMacOS
    {
        const string LIBDL = "libdl";

        [DllImport(LIBDL)]
        public static extern IntPtr dlopen(string path, int flags);

        [DllImport(LIBDL)]
        public static extern int dlclose(IntPtr handle);

        [DllImport(LIBDL)]
        public static extern IntPtr dlerror();

        [DllImport(LIBDL)]
        public static extern IntPtr dlsym(IntPtr handle, string name);
    }
}