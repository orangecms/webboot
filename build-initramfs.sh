#!/bin/sh

export GOOS=linux

u-root \
  -build bb \
  -files bzImage \
  -files webboot.sh \
  -files webboot/webboot:/bin/webboot \
  -files wifi:/bin/wifi
