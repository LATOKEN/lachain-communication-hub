# LACHAIN COMMUNICATION HUB

LibP2P based solution for P2P communication of LACHAIN nodes


#### Build

Gen protobuf files

``` 
cd cmd/protoc-gen-go-grpc && go install . && cd -
protoc   --go_out=Mgrpc/service_config/service_config.proto=/internal/proto/grpc_service_config:.   --go-grpc_out=Mgrpc/service_config/service_config.proto=/internal/proto/grpc_service_config:.   --go_opt=paths=source_relative   --go-grpc_opt=paths=source_relative   grpc/protobuf/message.proto
```




Build project
```
    go build -o hub main.go
```

Build shared library to use in other apps
```
    go build -o libhub.so embedded_hub.go
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
