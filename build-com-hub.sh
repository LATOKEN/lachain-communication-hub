SRC_PATH=D:/latoken/lachain-communication-hub
PACK_PATH=D:/latoken/lachain-hub
rm -r $PACK_PATH
mkdir -p $PACK_PATH
cd $SRC_PATH

# remove existing lib
rm -r ./lib

go get -d -v ./...
go build -o lib/win-x64/hub.dll -buildmode=c-shared embedded_hub.go

cd ./lib

mkdir -p ./linux-x64
cp ./win-x64/hub.dll ./linux-x64/libhub.so
cp ./win-x64/hub.h ./linux-x64/libhub.h

mkdir -p ./osx-x64
cp ./win-x64/hub.dll ./osx-x64/libhub.dylib
cp ./win-x64/hub.h ./osx-x64/libhub.h

cd ..

# remove old package
rm ./Lachain.CommunicationHub.Native.*.nupkg

dotnet restore Lachain.CommunicationHub.Net
dotnet build -c Release Lachain.CommunicationHub.Net
./nuget pack Lachain.CommunicationHub.Native.nuspec
./nuget push Lachain.CommunicationHub.Native.*.nupkg -Source $PACK_PATH

cd ./Lachain.CommunicationHub.Net

# remove old package
rm ./bin/Debug/Lachain.CommunicationHub.Net.*.nupkg

dotnet build Lachain.CommunicationHub.Net.sln
dotnet pack Lachain.CommunicationHub.Net.csproj
cd ..
./nuget push $SRC_PATH/Lachain.CommunicationHub.Net/bin/Debug/Lachain.CommunicationHub.Net.*.nupkg -Source $PACK_PATH


