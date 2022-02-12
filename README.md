# LACHAIN COMMUNICATION HUB

LibP2P based solution for P2P communication of LACHAIN nodes


#### Build

Gen protobuf files

``` 
cd cmd/protoc-gen-go-grpc && go install . && cd -
protoc   --go_out=Mgrpc/service_config/service_config.proto=/internal/proto/grpc_service_config:.   --go-grpc_out=Mgrpc/service_config/service_config.proto=/internal/proto/grpc_service_config:.   --go_opt=paths=source_relative   --go-grpc_opt=paths=source_relative   grpc/protobuf/message.proto
```



Build shared library to use in other apps
```
    go build -o libhub.so embedded_hub.go mock_main.go
```

#### Run

Relay-node

```
    ./hub -relay
```


Peer

```
    ./hub
```

Peer on custom port
```
    ./hub -port :50002
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
