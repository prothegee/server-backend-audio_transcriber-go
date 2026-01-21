#!/usr/bin/sh
docker rm -f server-backend-audio_transcriber-go;

docker run -d --name server-backend-audio_transcriber-go \
  -p 20202:20202 \
  server-backend-audio_transcriber-go;
