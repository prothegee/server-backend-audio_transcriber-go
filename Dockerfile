from golang:1.25-alpine

run apk add --no-cache \
    libc-dev libstdc++-dev \
    git \
    gcc g++ \
    cmake ninja \
    wget \
    pkgconf linux-headers \
    alsa-lib-dev portaudio-dev


workdir /repo

run git clone https://github.com/ggml-org/whisper.cpp --depth 1

workdir /repo/whisper.cpp
run cmake -S . -G "Ninja" -B build \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_INSTALL_PREFIX=/usr/local \
    -DBUILD_SHARED_LIBS=0 && \
    cmake --build build --config Release --target all -- -j$(nproc) && \
    cmake --install build --prefix=/usr/local


workdir /app

copy . .

run go mod download

env CGO_ENABLED=1
env GOOS=linux
env GOARCH=amd64
env CGO_LDFLAGS="-L/usr/local/lib -lwhisper -lggml -lggml-base -lggml-cpu -lm -lstdc++ -fopenmp"
env CGO_CFLAGS="-I/usr/local/include"
env PKG_CONFIG_PATH=/usr/lib/pkgconfig:/usr/local/lib/pkgconfig

run mkdir -p bin assets /llm
run go build -o bin/grpc_server/main ./cmd/grpc_server

run /repo/whisper.cpp/models/download-ggml-model.sh base.en /llm

RUN addgroup -g 1001 -S appuser && \
    adduser -u 1001 -S appuser -G appuser

copy ./config.grpc.json.template ./config.grpc.json
copy ./config.audio.json.template ./config.audio.json

run sed -i 's|"/path/to/llm/ggml-base.en.bin"|"/llm/ggml-base.en.bin"|' ./config.audio.json

run chown -R appuser:appuser /app
run chown -R appuser:appuser /llm

user appuser

expose 20202


workdir /app/bin/grpc_server
cmd ["./main"]
