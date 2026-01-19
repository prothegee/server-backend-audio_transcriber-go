#!/usr/bin/sh
set -e;

export CURRENT_DIR="$(pwd)";

cd "$CURRENT_DIR/cmd/audio_client";
go run .;

cd $CURRENT_DIR;
