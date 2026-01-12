#!/usr/bin/sh
set -e;

export CURRENT_DIR="$(pwd)";

export C_INCLUDE_PATH="$HOME/include";
export LIBRARY_PATH="$HOME/lib";

cd "$CURRENT_DIR/cmd/grpc_server";
go run .;

cd CURRENT_DIR;
