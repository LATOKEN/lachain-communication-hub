# LACHAIN COMMUNICATION HUB

LibP2P based solution for P2P communication of LACHAIN nodes


## Build

#### Native go libraries
Linux:

```
go build -o lib/linux-x64/libhub.so -buildmode=c-shared embedded_hub.go
```

Windows:

```
go build -o lib/win-x64/libhub.dll -buildmode=c-shared embedded_hub.go
```

MacOS:

```
go build -o lib/osx-x64/libhub.dylib -buildmode=c-shared embedded_hub.go
```

#### Lachain.CommunicationHub.Native
All go libraries from all 3 platforms should be present in lib folder as shown in previous step

```
nuget pack Lachain.CommunicationHub.Native.nuspec
```

#### Lachain.CommunicationHub.Net
```
cd Lachain.CommunicationHub.Net/
dotnet build Lachain.CommunicationHub.Net.sln
nuget pack Lachain.CommunicationHub.Net.csproj
```

#### Test

Build testnub:
```
    go build -o testhub testhub.go embedded_hub.go
```
Prepare local configs and script for test:
```
    go run ./testconfiggen.go -peerNumber 30
```
Start local test (file will be generated with config generator):
```
    bash ./starthubs.sh
```

Prepare remote configs and scripts:
```
go run ./testconfiggen.go -peerNumber 22 -ips 116.203.75.72,178.128.113.97,165.227.45.119,206.189.137.112,157.245.160.201,95.217.6.171,88.99.190.191,94.130.78.183,94.130.24.163,94.130.110.127,94.130.110.95,94.130.58.63,88.99.86.166,88.198.78.106,88.198.78.141,88.99.126.144,88.99.87.58,95.217.6.234,95.217.12.226,95.217.14.117,95.217.17.248,95.217.12.230 -port 7077 -key ~/.ssh/gladcow.pub
```
Deploy files to server - deploy.sh,  start test - starthubs.sh,  remove files from servers - remove.sh