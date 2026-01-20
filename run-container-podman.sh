#!/usr/bin/sh
podman rm -f server-backend-audio_transcriber-go;

podman run -d --name server-backend-audio_transcriber-go \
  -p 20202:20202 \
  server-backend-audio_transcriber-go;
