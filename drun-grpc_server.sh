#!/usr/bin/sh
set -e;

export CURRENT_DIR="$(pwd)";

cd "$CURRENT_DIR/cmd/grpc_server";
go run .;

cd $CURRENT_DIR;
