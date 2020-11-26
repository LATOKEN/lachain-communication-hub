#!/bin/bash

docker build -t lachain-communication-hub .
mkdir -p "lib/linux-x64"
docker run --rm -v "$(pwd)/lib/linux-x64":/opt/lib lachain-communication-hub
