#!/usr/bin/sh
set -e;

export CURRENT_DIR="$(pwd)";

export C_INCLUDE_PATH="$HOME/include";
export LIBRARY_PATH="$HOME/lib";

cd "$CURRENT_DIR/cmd/audio_client_by_port";
go run .;

cd CURRENT_DIR;
