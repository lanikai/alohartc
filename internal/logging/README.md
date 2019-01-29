# logging #

This package is meant to replace usages of the `log` package from the Go
standard library. It assigns to each log statement both a verbosity level and a
tag. At runtime, one can choose via an environment variable which log statements
actually get printed. This provides better control of signal-to-noise during
debugging, since verbose logging can be enabled for only one or a few packages.


## API ##

The recommended usage pattern is to declare a package-wide `log` variable,
derived from `logging.DefaultLogger`. For example, somewhere in the package
`mypackage` you would have:

    var log = logging.DefaultLogger.WithTag("mypackage")

This defines the tag that will be associated with all log statements within this
package. Then from another package file `afile.go` you might write:

    log.Debug("This is a test message")
    ...
    log.Error("Uh oh: %v", err)

If Debug-level logging is enabled for `mypackage` (see below), these log
statements are printed as

    2019-01-01 12:34:56.789 D/mypackage[afile.go:33] This is a test message
    2019-01-01 12:34:56.789 E/mypackage[afile.go:42] Uh oh: i/o error

However if the configured logging level for `mypackage` is Info or lower, then
only the Error message will be printed.


## Levels ##

Log levels are integers, with larger values corresponding to greater verbosity.
The lowest allowed level is `Error`, followed by `Warn`, `Info`, and `Debug`. In
addition to these named levels, you may make use of up to eight additional
"trace levels", numbered 2 through 9 inclusive. (Trace level 0 is equivalent to
`Info`, and trace level 1 is `Debug`.)

The level of an individual log statement is determined by the API call used:

    log.Error("We have a serious problem")
    log.Warn("Something unexpected happened, but I'm sure it will be okay")
    log.Info("All going according to plan, captain")
    log.Debug("Hey here's something you might be interested in")
    log.Trace(4, "TMI my friend, TMI")

These functions are all in the style of `fmt.Printf()`, i.e. they take a format
string plus a variable number of arguments. (Unlike the standard `log` package,
there are no `Print` or `Println` variants.)


The logging level enabled for a given package is determined at runtime by the
the `LOGLEVEL` environment variable. This is a comma-separated list of
`tag=level` pairs, e.g.

    LOGLEVEL="pkgone=debug,pkgtwo=warn"

As a special case, the `tag=` prefix may be omitted, in which case the specified
level is used as the default for all loggers. The example below sets the log
level to Warn for all packages:

    LOGLEVEL="warn"


## Compatibility with `log` package ##

Naming the package-wide variable `log` makes it act as a nearly drop-in
replacement for most usages of the standard `log` package. Migration from the
latter typically requires only two steps:

1. Declare a package-wide `log` variable in one of the package files.
2. Remove `import "log"` statements from all package files.

The `logging.Logger` type defines compatibility functions `Print*`, `Panic*`,
and `Fatal*` to facilitate this migration. However it is recommended to use the
explicit leveled API (`Error`, `Warn`, etc.) for new code.
