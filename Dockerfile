FROM golang:1.15.0-buster
WORKDIR /go/src/app
COPY . .
RUN go get -d -v ./...
RUN go build -o libhub.so -buildmode=c-shared embedded_hub.go
ENTRYPOINT ["bash", "-c", "cp /go/src/app/libhub.so /opt/lib/libhub.so"]
