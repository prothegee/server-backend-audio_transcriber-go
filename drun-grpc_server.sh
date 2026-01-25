#!/usr/bin/sh
set -e;

export CURRENT_DIR="$(pwd)";

WHISPER_LIB="libwhisper.a" # libwhisper.a or libwhisper.so
WHISPER_LIB_PATH=""
WHISPER_WHEREIS_OUTPUT=$(whereis $WHISPER_LIB)

if echo "$WHISPER_WHEREIS_OUTPUT" | grep -q ":"; then
    LIB_FILE=$(echo "$WHISPER_WHEREIS_OUTPUT" | cut -d: -f2 | awk '{print $1}' | head -n1)
    if [ -n "$LIB_FILE" ] && [ -f "$LIB_FILE" ]; then
        WHISPER_LIB_PATH=$(dirname "$LIB_FILE")
    fi
fi

if [ -z "$WHISPER_LIB_PATH" ]; then
    echo "error: $WHISPER_LIB not found";
    exit 1;
fi

export CGO_LDFLAGS="-L$WHISPER_LIB_PATH -lwhisper -lggml -lggml-base -lggml-cpu -lm -lstdc++ -fopenmp"

cd "$CURRENT_DIR/cmd/grpc_server";
go run .;

cd $CURRENT_DIR;
