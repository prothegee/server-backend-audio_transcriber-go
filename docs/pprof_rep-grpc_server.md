# pprof rep grpc server

__*2026 01 12 | #1*__
```
File: grpc_server
Build ID: a4fe13fe2824b3e4c94758c922d62ee378314c71
Type: inuse_space
Time: 2026-01-12 14:23:51 WIB
Entering interactive mode (type "help" for commands, "o" for options)
(pprof) top
Showing nodes accounting for 5205.44kB, 100% of 5205.44kB total
Showing top 10 nodes out of 52
      flat  flat%   sum%        cum   cum%
 1089.33kB 20.93% 20.93%  1089.33kB 20.93%  main.bytesToFloat32
    1026kB 19.71% 40.64%     1026kB 19.71%  runtime.allocm
 1025.75kB 19.71% 60.34%  1025.75kB 19.71%  sync.(*Pool).pinSlow
  528.17kB 10.15% 70.49%   528.17kB 10.15%  main.(*server).TranscribeStream.func2
  512.14kB  9.84% 80.33%   512.14kB  9.84%  google.golang.org/grpc/resolver.Register
  512.03kB  9.84% 90.16%   512.03kB  9.84%  net.(*file).getLineFromData
  512.02kB  9.84%   100%   512.02kB  9.84%  main.main
         0     0%   100%   512.88kB  9.85%  golang.org/x/net/http2.(*Framer).ReadFrameForHeader
         0     0%   100%   512.88kB  9.85%  golang.org/x/net/http2.(*Framer).readMetaFrame
         0     0%   100%   512.88kB  9.85%  golang.org/x/net/http2/hpack.(*Decoder).Write
```

<br>

---

###### end of pprof rep grpc server
