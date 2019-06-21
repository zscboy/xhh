#!/bin/bash
set -e

go build

VersionCode=$(./xhmj -v)_$(date +%Y%m%d%H%M%S)
tar -zcvf xhmj_$VersionCode.tar.gz xhmj
echo "build success"
