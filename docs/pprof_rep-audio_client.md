
# pprof rep audio client

__*2026 01 12 | #1*__
```
File: audio_client
Build ID: 7b33534c9c4b1a49744b4cc9d5bbd69b546e637d
Type: inuse_space
Time: 2026-01-12 12:11:33 WIB
Entering interactive mode (type "help" for commands, "o" for options)
(pprof) top
Showing nodes accounting for 2052.30kB, 100% of 2052.30kB total
Showing top 10 nodes out of 17
      flat  flat%   sum%        cum   cum%
  515.19kB 25.10% 25.10%   515.19kB 25.10%  html/template.map.init.0
     513kB 25.00% 50.10%      513kB 25.00%  runtime.allocm
  512.05kB 24.95% 75.05%   512.05kB 24.95%  context.(*cancelCtx).Done
  512.05kB 24.95%   100%   512.05kB 24.95%  main.main
         0     0%   100%   512.05kB 24.95%  google.golang.org/grpc/internal/transport.NewHTTP2Client.func4
         0     0%   100%   515.19kB 25.10%  html/template.init
         0     0%   100%   515.19kB 25.10%  runtime.doInit (inline)
         0     0%   100%   515.19kB 25.10%  runtime.doInit1
         0     0%   100%  1027.25kB 50.05%  runtime.main
         0     0%   100%      513kB 25.00%  runtime.mstart
```

<br>

---

###### end of pprof rep audio client
