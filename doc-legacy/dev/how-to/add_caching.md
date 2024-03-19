# How to add caching

## Background

Sourcegraph is backed by two Redis caches, `redis-cache` and `redis-store`. 

## Usage

Engineers are welcome to cache data as they see fit.

The cache instances are instantiated in [`internal/redispool`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/redispool/redispool.go).
You can use the [`rcache` package](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@main/-/blob/internal/rcache/rcache.go) to read and write data to and from `redis-cache`, which you are also welcome to add to.

## Troubleshooting local redis access

A simple way to test your caching locally is through the `redis-cli`. You can install it using `brew` if you do not have it:

```shell
brew install redis 
```

You might run into the following error with `sg` after installing:

```
❌ Start Redis
   failed to write to Redis at 127.0.0.1:6379: MISCONF Redis is configured to save RDB snapshots, but it's currently unable to persist to disk. Commands that may modify the data set are disabled, because this instance is configured to report errors during writes if RDB snapshotting fails (stop-writes-on-bgsave-error option). Please check the Redis logs for details about the RDB error.
```

Potential solutions for this are:

1. Restarting Redis (e.g. `brew services restart redis`)
2. Changing the config to stop writes:

```shell
redis-cli 
> config set stop-writes-on-bgsave-error no
> exit
```

## Helpful links

* [Redis commands](https://redis.io/commands/)
