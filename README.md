# backend audio transcriber go

A showcase audio transcriber server in golang using audio ml model & leverage protocol buffer with grpc.

<br>

__TL;DR__
```
It all begin when I watch coworker start live streaming in one of the biggest social media platform,
afer a few minutes, they got warning violation notification.

As far as they know, they believed that they don't violate their community rules,
since they also believed their video presentation are appropriate.

After couple attempts, the violation appear again, but now we realized that violation notification appear after they says something.
```

<!-- RESERVED: extra information -->

<br>

---

idea flow
1. > client/end-user send the audio -> server check & processing the audio -> send response

2. > if the audio process contain *forbidden* keywords -> do something (warn, err, etc.)

<br>

---

## important

1. run and tested in:

    - go version go1.25.5 X:nodwarf5 linux/amd64

2. if `config.audio.json` & `config.grpc.json` doesn't exists, copy and paste those file from .json.template to .json

3. you need to build and expose the library from [whisper.cpp](https://github.com/ggerganov/whisper.cpp) and install the model:
    - after the installation, you need to export the include path and include lib directory, see [dbuild.sh](./dbuild.sh#L9)
    - without that you can't run this project
    - for the ml whisper model, you can configure in [whisper.model field](./config.audio.json.template#L3)
    
4. to run the server or client:
    - for instance without defining a custom port use:
        - [drun-grpc_server.sh](./drun-grpc_server.sh)
        - [drun-audio_client.sh](./drun-audio_client.sh)
    - if you want to use manual asignin port use:
        - [drun-grpc_server-by_port.sh](./drun-grpc_server-by_port.sh)
        - [drun-audio_client-by_port.sh](./drun-audio_client-by_port.sh)
        
5. use proper model and check the forbidden keywords

<br>

---

## problem that we might encounter

1. hardware resource exhaustion:

    1. try to adjust:
        - `sending_ticker` in audio_client json config
        - `audio_processing` in grpc_server json config
        
    2. set the ammount of:
        - audio_client:
            - `framesPerBuf`
            - `audioBufferChannelSize`
            
```
currently by default if you using *by_port instance,
on 12 threads able to run 4 client and 4 server
```
![fig_1](./docs/img/fig_1.png)

<!-- a record doc for staging server, stress test or any actual report -->

<br>

---

### memory heap profiler report

- [grpc server pprof](./docs/pprof_rep-grpc_server.md)
- [audio client pprof](./docs/pprof_rep-audio_client.md)

<br>

---

### extra

@prothegee
```
if you have better improvement both from the pogram and architecture, I would love to know/read that!
```

<br>

---

###### end of readme
