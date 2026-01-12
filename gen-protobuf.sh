#!/usr/bin/bash
set -e;

export PWD="$(pwd)";
export PROTO_DIR="$PWD/protobuf";

# generate location proto
# manual: protoc ./protobuf/audio.proto --go_out=THIS_DIR --go-grpc_out=THIS_DIR
protoc "$PROTO_DIR/audio.proto" \
    --go_out="$PWD" \
    --go-grpc_out="$PWD" \
    --proto_path="$PROTO_DIR";
