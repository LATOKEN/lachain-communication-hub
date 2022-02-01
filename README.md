# LACHAIN COMMUNICATION HUB

LibP2P based solution for P2P communication of LACHAIN nodes


## Build

#### Native go libraries
Linux:

`go build -o lib/linux-x64/libhub.so -buildmode=c-shared embedded_hub.go`

Windows:

`go build -o lib/wiun-x64/libhub.dll -buildmode=c-shared embedded_hub.go`

MacOS:

`go build -o lib/osx-x64/libhub.dylib -buildmode=c-shared embedded_hub.go`

#### Lachain.CommunicationHub.Native
All go libraries from all 3 platforms should be present in lib folder as shown in previous step

`nuget pack Lachain.CommunicationHub.Native.nuspec`

#### Lachain.CommunicationHub.Net
```
cd Lachain.CommunicationHub.Net/
dotnet build Lachain.CommunicationHub.Net.sln
nuget pack Lachain.CommunicationHub.Net.csproj
```
