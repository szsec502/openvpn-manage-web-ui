#!/bin/bash

set -e

time docker run \
    -v "$PWD/../":/go/src/github.com/tyzbit/openvpn-web-ui \
    --rm \
    -w /usr/src/myapp \
    tyzbit/beego:1.9.4 \
    sh -c "cd /go/src/github.com/tyzbit/openvpn-web-ui/ && bee version && bee pack -exr='^vendor|^data.db|^build|^README.md|^docs'"
