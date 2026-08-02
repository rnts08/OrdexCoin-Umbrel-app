#!/bin/bash

docker run --rm -d --name ordexcoin-web \
  --network host \
  -v $HOME/.ordexcoin:/data:ro \
  --entrypoint /usr/local/bin/ordexcoin-web \
  ordexcoin-web:local \
  -datadir /data -listen 0.0.0.0:3000
