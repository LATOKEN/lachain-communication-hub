using System;

namespace Lachain.CommunicationHub.Net
{
    class SymbolNameAttribute : Attribute
    {
        public readonly string Name;

        public SymbolNameAttribute(string name)
        {
            Name = name;
        }
    }
}