#!/bin/bash
set -e

go build

VersionCode=$(./xhh -v)_$(date +%Y%m%d%H%M%S)
tar -zcvf xhh_$VersionCode.tar.gz xhh
echo "build success"