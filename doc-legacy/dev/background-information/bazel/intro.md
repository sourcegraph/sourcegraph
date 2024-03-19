# Bazel intro

## Why do we need a build system?

Building Sourcegraph is a non-trivial task, as it not only ships a frontend and a backend, but also a variety of third parties and components that makes the building process complicated, not only locally but also in CI. Historically, this always have been solved with ad-hoc solutions, such as shell scripts, and caching in various point of the process.

But we're using languages that traditionally don't require their own build systems right? Go and Typescript have their own ecosystem and solve those problems each with their own way right? Yes indeed, but this also means they are never aware of each other and anything linking them requires to be implemented manually, which what we've done so far. Because the way our app is built, as a monolith, it's not trivial to detect things such as the need to rebuild a given Docker image because a change was made in another package, because there is nothing enforcing this structurally in the codebase. So we have to rebuild things, because there is doubt.

On top of that, whenever we attempt at building our app, we also need to fetch many third parties from various locations (GitHub releases, NPM, Go packages, ...). While most of the time, it's working well, any failure in doing so will result in failed build. This may go unnoticed when working locally, but on CI, this can prevent us to build our app for hours at times if we can't fetch the dependency we need, until the problem is resolved upstream. This makes us very dependent on the external world.

In the end, it's not what composes our application that drives us to use a build system, but instead the size it has reached after years of development. We could solve all these problems individually with custom solutions, that would enable us to deterministically say that we need to build X because Y changed. But guess what? The result would pretty much look like a build system. It's a known problem and solutions exists in the wild for us to use.

Finally, build systems provides additional benefits, especially on the security side. Because a build system is by definition aware of every little dependency, we can use that to ensure we react swiftly to CVEs (Common Vulnerabilities and Exposures) produce SBOMs (Software Bill of Materials) for our customers to speed up the upgrade process.

## Why Bazel?

Bazel is the most used build system in the world that is fully language agnostic. It means you can build whatever you want with Bazel, from a tarball containing Markdown files to an iOS app. It's like Make, but much more powerful. Because of its popularity, it means its ecosystem is the biggest and a lot have been written for us to use already.

We could have used others, but that would translate in having to write much more. Building client code for example is a delicate task, because of the complexity of targeting browsers and the pace at which its ecosystem is evolving. So we're avoiding that by using a solution that has been battle tested and proved to still work at scale hundred times bigger than ours and smaller than us.

## What is Bazel?

Bazel sits in between traditional tools that build code, and you, similarly to how Make does one could say. At it's core, Bazel enables you to describe a hierarchy of all the pieces needed to build the application, plus the steps required to build one based on the others.

### What a build system does

Let's take a simple example: we are building a small tool that is written in Go and we ship it with `jq` so our users don't need to install it to use our code. We want our release tarball to contain our app binary, `jq` and our README.

Our codebase would look like this:

```
- README.md
- main.go
- download_jq.sh
```

The result would look like this:

```
- app.tar.gz
  - ./app
  - ./jq
  - README.md
```

To built it we need to perform the following actions:

1. Build `app` with `go build ...`
1. Run `./download_jq.sh` to fetch the `jq` binary
1. Create a tarball containing `app`, `jq` and `README.md`

If we project those actions onto our filetree, it looks like this (let's call it an _action graph_):

```
- app.tar.gz
  - # tar czvf app.tar.gz .
    - ./app
      - # go build main.go -o app
    - ./jq
      - # ./download_jq.sh
    - README.md
```

We can see how we have a tree of _inputs_ forming the final _output_ which is `app.tar.gz`. If all _inputs_ of a given _output_ didn't change, we don't need to build them again right? That's exactly the question that a build system can answer, and more importantly *deterministically*. Bazel is going to store all the checksums of _inputs_ and _outputs_ and will perform only what's required to generate the final _output_.

If our Go code did not change, we're still using the same version of `jq` but the README changed, do I need to generate a new tarball? Yes because the tarball depends on the README as well. If neither changed, we can simply keep the previous tarball. If we do not have Bazel, we need to provide a way to ensure it.

As long as Bazel's cache is warm, we'll never need to run `./download_jq.sh` to download `jq` again, meaning that even if GitHub is down and we can't fetch it, we can still build our tarball.

For Go and Typescript, this means that every dependency, either a Go module or a NPM package will be cached, because Bazel is aware of it. As long as the cache is warm, it will never download it again. We can even tell Bazel to make sure that the checksum of the `jq` binary we're fetching stays the same. If someone were to maliciously swap a `jq` release with a new one, Bazel would catch it, even it was the same exact version.

### Tests are outputs too.

Tests, whether it's a unit test or an integration tests, are _outputs_ when you think about it. Instead of being a file on disk, it's just green or red. So the same exact logic can be applied to them! Do I need to run my unit tests if the code did not change? No you don't, because the _inputs_ for that test did not change.

Let's say you have integration tests driving a browser querying your HTTP API written in Go. A naive way of representing this would be to say that the _inputs_ for that e2e test are the source for the tests. A better version would be to say that the _inputs_ for this tests are also the binary powering your HTTP API. Therefore, changing the Go code would trigger the e2e tests to be ran again, because it's an _input_ and it changed again.

So, building and testing is in the end, practically the same thing.

### Why is Bazel frequently mentioned in a negative light on Reddit|HN|Twitter|... ?

Build systems are solving a complex problem. Assembling a deterministic tree of all the _inputs_ and _outputs_ is not an easy task, especially when your project is becoming less and less trivial. And to enforce it's properties, such as hermeticity and being deterministic, Bazel requires both a "boil the ocean first" approach, where you need to convert almost everything in your project to benefit from it and to learn how to operate it. That's quite the upfront cost and a lot of cognitive weight to absorb, naturally resulting in negative opinions.

In exchange for that, we get a much more robust system, resilient to some unavoidable problems that comes when building your app requires to reach the outside world.

### Sandboxing

Bazel ensures it's running hermetically by sandboxing anything it does. It won't build your code right in your source tree. It will copy all of what's needed to build a particular _target_ in a temporary directory (and nothing more!) and then apply all the rules defined for these _targets_.

This is a *very important* difference from doing things the usual way. If you didn't tell Bazel about an _input_, it won't be built/copied in/over the sandbox. So if your tests are relying testdata for examples, Bazel must be aware of it. This means that it's not possible to change the _outputs_ by accident because you created an additional file in the source tree.

So having to make everything explicit means that the buildfiles (the `BUILD.bazel` files) need to be kept in sync all the time. Luckily, Bazel comes with a solution to automate this process for us.

### Generating buildfiles automatically

Bazel ships with a tool named `Gazelle` whose purpose is to take a look at your source tree and to update the buildfiles for you. Most of the times, it's going to do the right thing. But in some cases, you may have to manually edit the buildfiles to specify what Gazelle cannot guess for you.

Gazelle and Go: It works almost transparently with Go, it will find all your Go code and infer your dependencies from inspecting your imports. Similarly, it will inspect the `go.mod` to lock down the third parties dependencies required. Because of how well Gazelle-go works, it means that most of the time, you can still rely on your normal Go commands to work. But it's recommended to use Bazel because that's what will be used in CI to build the app and ultimately have the final word in saying if yes or no a PR can be merged. See the [cheat sheet section](index.md#bazel-cheat-sheet) for the commands.

Gazelle and the frontend: see [Bazel for Web bundle](./web.md).
