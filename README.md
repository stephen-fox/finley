# finley
Package finley provides functionality that extends the CLI implementation of
ILSpy (https://github.com/icsharpcode/ILSpy). ILSply is an open source .NET
decompiler. It is normally invoked as a GUI application. However, a CLI
implementation ('ilspycmd') provides a better platform for automation.

The primary features of finley include recursive decompilation of .NET
binaries  in a given directory, duplicate .NET binary avoidance, and
configurable concurrent decompilation of .NET binaries.

## Requirements
Before using finley, you must install `ilspycmd`. This requires the .NET Core
SDK. If you are a chocolatey user, you can install this by running:
```
choco install dotnetcore-sdk -y
```

After installing .NET Core SDK, you can install `ilspycmd` by executing the
following command:
```
dotnet tool install -g ilspycmd --version 6.0.0.5559-preview2
```

Note: the version number of `ilspycmd` is explicitly provided. This is because,
at the time of writing this post, the current non-preview version of `ilspycmd`
does not support the shipping version of .NET Core.

## Usage
Typically, finley is used to decompile a directory containing .NET binaries:
```
finley -d some-directory
```

If you would like to recursively decompile all of the binaries in a given
directory (i.e., include sub-directories), you can add the `-r` argument:
```
finley -d some-directory -r
```

For additional arguments, run with the `-h` option. This will produce a list of
arguments and an explanation of their effects:
```
  -allow-duplicates
    	Decompile file even if its hash has already been encountered
  -d string
    	The directory to search for DLLs
  -e string
    	Comma separated list of file extensions to search for (default ".dll")
  -ilspy string
    	The 'ilspycmd' binary to use (default "ilspycmd")
  -no-ilspy-errors
    	Exit if ILSpy fails to decompile a file
  -num-workers int
    	Number of .NET decompiler instances to run concurrently (default n)
  -o string
    	The output directory. Creates a new directory if not specified
  -r	Scan recursively
  -respect-file-case
    	Respect filenames' case when matching their extensions
```

## Building from source
You can use any of the following methods to build the application:

- `go build cmd/finley/main.go` - Build the application
- `build.sh` - A simple wrapper around 'go build' that saves build artifacts
to `build/` and sets a version number in the compiled binary. This script
expects a version to be provided by setting an environment variable
named `VERSION`
- `buildwin.sh` - Build the application for Windows (since that seems like the
most common OS this tool would be used on)
