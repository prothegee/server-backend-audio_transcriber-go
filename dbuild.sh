#!/usr/bin/sh
set -e;

grpc_cfg_file="$(pwd)/config.grpc.json";
audio_cfg_file="$(pwd)/config.audio.json";

if ! [ -f "$grpc_cfg_file" ]; then
    echo "\"$grpc_cfg_file\" doesn't exists";
    echo "try to copy \"$grpc_cfg_file.template\" to \"$grpc_cfg_file\" first";
    exit 1;
fi
if ! [ -f "$audio_cfg_file" ]; then
    echo "\"$audio_cfg_file\" doesn't exists";
    echo "try to copy \"$audio_cfg_file.template\" to \"$audio_cfg_file\" first";
    exit 1;
fi

# whisper library
WHISPER_LIB="libwhisper.a" # libwhisper.a or libwhisper.so
WHISPER_LIB_PATH=""
WHISPER_WHEREIS_OUTPUT=$(whereis $WHISPER_LIB)

# extract dirname after `: `
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

# set cho flags
export CGO_LDFLAGS="-L$WHISPER_LIB_PATH -lwhisper -lggml -lggml-base -lggml-cpu -lm -lstdc++ -fopenmp"

mkdir -p bin;

CURRENT_DIR="$(pwd)";

# grpc_server
GRPC_SERVER_SOURCE="$CURRENT_DIR/cmd/grpc_server";
GRPC_SERVER_TARGET="$CURRENT_DIR/bin/grpc_server/main";

echo "building: $GRPC_SERVER_SOURCE";
echo "- target: $GRPC_SERVER_TARGET";
go build -o $GRPC_SERVER_TARGET $GRPC_SERVER_SOURCE;

# audio_client
AUDIO_CLIENT_SOURCE="$CURRENT_DIR/cmd/audio_client";
AUDIO_CLIENT_TARGET="$CURRENT_DIR/bin/audio_client/main";

echo "building: $AUDIO_CLIENT_SOURCE";
echo "- target: $AUDIO_CLIENT_TARGET";
go build -o $AUDIO_CLIENT_TARGET $AUDIO_CLIENT_SOURCE;
