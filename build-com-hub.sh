SRC_PATH=~/latoken/lachain-communication-hub
PACK_PATH=~/latoken/lachain-hub
rm -r $PACK_PATH
mkdir -p $PACK_PATH
cd $SRC_PATH

# remove existing lib
rm -r ./lib

go get -d -v ./...
go build -o lib/linux-x64/libhub.so -buildmode=c-shared embedded_hub.go

cd ./lib

mkdir -p ./win-x64
cp ./linux-x64/libhub.so ./win-x64/hub.dll
cp ./linux-x64/libhub.h ./win-x64/hub.h

mkdir -p ./osx-x64
cp ./linux-x64/libhub.so ./osx-x64/libhub.dylib
cp ./linux-x64/libhub.h ./osx-x64/libhub.h

cd ..

# remove old package
rm ./Lachain.CommunicationHub.Native.*.nupkg

dotnet restore Lachain.CommunicationHub.Net
dotnet build -c Release Lachain.CommunicationHub.Net
nuget pack Lachain.CommunicationHub.Native.nuspec
nuget push Lachain.CommunicationHub.Native.*.nupkg -Source $PACK_PATH

cd ./Lachain.CommunicationHub.Net

# remove old package
rm ./bin/Debug/Lachain.CommunicationHub.Net.*.nupkg

dotnet build Lachain.CommunicationHub.Net.sln
dotnet pack Lachain.CommunicationHub.Net.csproj
nuget push ./bin/Debug/Lachain.CommunicationHub.Net.*.nupkg -Source $PACK_PATH


