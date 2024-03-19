# How to do one-off profiling for dogfood and production using pprof

Go has built-in support for a sampling profiler [`pprof`](https://github.com/google/pprof),
which can be used to investigate CPU usage, heap usage and more.

You can obtain and examine a profile from a running Sourcegraph instance as follows:

1. Select the right Kubernetes context using [`kubectx`](https://github.com/ahmetb/kubectx) or `kubectl`.
2. Set up port-forwarding for the pod you're interested in profiling using [`k9s`](https://k9scli.io/) or `kubectl`. For example:
   ```
   kubectl port-forward precise-code-intel-worker-0000000000-00000 6061:6060
   ```
   This will map port 6060 on the pod to port 6061 on your machine.
3. Record a profile.
   ```
   # Sample CPU usage for 60 seconds
   go tool pprof -seconds 60 http://localhost:6061/debug/pprof/profile
   # Sample heap usage for 60 seconds
   go tool pprof -seconds 60 http://localhost:6061/debug/pprof/heap
   ```
   This will save the output to a temporary file. (Or you can specify a path using `-output <path>`.)
4. Examine the generated using `go tool pprof`:
   ```
   # in the web UI
   go tool pprof -http :9999 /path/to/profile.pb.gz
   # in a REPL
   go tool pprof /path/to/profile.pb.gz
   ```
   The web UI supports visualizing the output as a flamegraph, as a call graph with weighted edges, and more.

Other resources:
* [(Blog post) Profiling Go tools with pprof](https://jvns.ca/blog/2017/09/24/profiling-go-with-pprof/)
* [pprof documentation](https://github.com/google/pprof/blob/master/doc/README.md)
* [net/http/pprof documentation](https://pkg.go.dev/net/http/pprof)
* [runtime/pprof documentation](https://pkg.go.dev/runtime/pprof)
* [How to enable continuous profiling in production](./profiling_continuous.md)
