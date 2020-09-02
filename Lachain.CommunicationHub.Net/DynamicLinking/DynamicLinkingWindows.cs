using System;
using System.Runtime.InteropServices;

namespace Lachain.CommunicationHub.Net.DynamicLinking
{
    // Taken from: https://github.com/MeadowSuite/Secp256k1.Net/blob/master/Secp256k1.Net/DynamicLinking/DynamicLinkingWindows.cs
    static class DynamicLinkingWindows
    {
        const string KERNEL32 = "kernel32";

        [DllImport(KERNEL32, SetLastError = true)]
        public static extern IntPtr LoadLibrary(string path);

        [DllImport(KERNEL32, SetLastError = true)]
        public static extern int FreeLibrary(IntPtr module);

        [DllImport(KERNEL32, SetLastError = true, CharSet = CharSet.Ansi, ExactSpelling = true)]
        public static extern IntPtr GetProcAddress(IntPtr module, string procName);
    }
}