# Hotreload
Ability to replace existing functions and methods on-the-fly for Go.

# How it works
It's magic! Based on plugins and some intelligent (not really) on-the-fly code rewrite.

# Dependencies
You must have go installed (obviously) and `goimports` needs to be reachable from your `$PATH`.

# What kind of live code reload is supported?
It is only possible to live-reload code of existing functions and methods provided the following conditions are met:

1. Functions can only use symbols that are declared in other packages. E.g. it's fine to call `flag.Parse()`, but not fine to accept a struct type that is defined in a current package (even if it is public).
2. Methods must specify a **public** type as their receiver. The method itself can be private.
3. Methods can only call **public** methods and access **public** fields of their receiver.

# Usage
1. Download version of go that supports plugins (1.8+ for Linux, 1.10+ for macOS, not yet supported on Windows)
2. Run the example script in the separate console: `./cmd/live/run.sh`. This will run an example web-server on port `:8080` that has two web handlers: `/get` that returns counter value and `/increment` that increments a counter but does not return it.
3. Try running `$ curl 'http://127.0.0.1:8080/increment'` to increment the in-memory counter.
4. Try running `$ curl 'http://127.0.0.1:8080/get'` to get the value of the in-memory counter.
5. Without closing the console with `run.sh`, edit the contents of `cmd/live/subpkg/pkg.go` and see what happens.
6. After you see the line `Hot reload was successful`, try accessing `/increment` and `/get` again.
7. If it worked, it's magic, right?

# How to use for your own application
Please remember that the only purpose of this package is to demonstrate that live reloading *is possible*, and it's not advisable to run anything like this in production. The intention of this package is to speed up development of go code in some cases, especially for stateful applications.

So, in order to use the package in your development environment, you need to add the `go hot.ReloaderLoop()` somewhere close to the beginning in your `main()` function:

```golang
import hot "github.com/YuriyNasretdinov/hotreload"

func main() {
  go hot.ReloaderLoop()
  // the rest of your code
}
```

It will start reading new plugin names from stdin so you can't use stdin in your app for anything else.

After that, you need to write some wrapper script that would be close to `./cmd/live/run.sh`:

```sh
#!/bin/sh
set -e
export GOPATH=$(go env GOPATH)

# Installing "hot" command line tool as a binary
go install github.com/YuriyNasretdinov/hotreload/cmd/hot

# Launching "hot" with two steps:
# 1. Compiling your program.
# 2. Launching it.

# You need to replace "<the directory with your application>" with a directory where changes
# will be  monitored.
# You can just set it to watch "$GOPATH/src" as a whole but it might fail to open
# so many directories at once.

# "<code to build your app>" is usually "go install <your_packages>"
# "<launch your app>" is usually "$GOPATH/bin/my_app".
# Note that the value of $GOPATH inside the shell command
# will be different from your normal GOPATH and
# it is important that you leave it in single quotes.

$GOPATH/bin/hot \
    -watch=$GOPATH/src/<the directory with your application> \
    sh -c 'set -e -x;\
    <code to build your app>;\
    <launch your app>'
```

Then, if you stick to the rules described below and only change the code of existing functions and methods, it should update code of your application on-the-fly!
**Note:** Debuggers probably won't work well in conjuction with this implementation of hot code reload.

# Can I use this in production?
Theoretically, yes! Hot code reload is based on https://github.com/YuriyNasretdinov/golang-soft-mocks which is memory- and thread-safe (which is not true for much more popular https://github.com/bouk/monkey). Provided you're fine with loading Go plugins on the fly in your production application and your changes are limited to what is described in "Examples of functions and methods that can be live-reloaded", it should be possible. It is probably not a good idea anyway because plugins cannot be unloaded from memory and if you live-reload your code in production too much, you will eventually run out of memory and waste a lot of resources.

# Examples of functions and methods that can be live-reloaded

## Function only accepts types from other packages
```golang
// good
func printTime(t time.Time) {
  log.Printf("Time: %s", t)
}
```

## Function only accepts primitive types
```golang
// good
func add2(arg int64) int64 {
  return arg + 2
}
```

## Function references only public variables from other packages
```golang
// good
func printOsArgs() {
  fmt.Printf("os.Args: %+v", os.Args)
}
```

## Method only calls other public methods for the same public receiver
```golang
// good
func (e *Example) callOtherMethod() {
  e.OtherMethod()
}
```

# Examples of functions and methods that cannot be live-reloaded
## Function accepts types defined in the same package
```golang
// bad, can't accept types from the same package
func printMyOwnTime(t Time) {
  log.Printf("Time: %s", t)
}
```

## Function accepts types defined in the same package
```golang
// bad, can't accept types from the same package
func incrementCounter(c *Counter) {
  (*c)++
}
```


## Method calls private methods
```golang
// bad, can't call private methods
func (c *Counter) Increment() {
  c.doIncrement()
}
```
