#!/usr/bin/bash
set -e;

mkdir -p bin;

export CURRENT_DIR="$(pwd)";

# grpc_server
export GRPC_SERVER_SOURCE="$CURRENT_DIR/cmd/grpc_server";
export GRPC_SERVER_TARGET="$CURRENT_DIR/bin/grpc_server/main";

echo "building: $GRPC_SERVER_SOURCE";
echo "- target: $GRPC_SERVER_TARGET";
go build -o $GRPC_SERVER_TARGET $GRPC_SERVER_SOURCE;

# audio_client
export AUDIO_CLIENT_SOURCE="$CURRENT_DIR/cmd/audio_client";
export AUDIO_CLIENT_TARGET="$CURRENT_DIR/bin/audio_client/main";

echo "building: $AUDIO_CLIENT_SOURCE";
echo "- target: $AUDIO_CLIENT_TARGET";
go build -o $AUDIO_CLIENT_TARGET $AUDIO_CLIENT_SOURCE;
