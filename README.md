# finley
finley makes it easy to quickly decompile several .NET binaries concurrently
without involving a GUI.

It does this by extending the CLI implementation of ILSpy
(https://github.com/icsharpcode/ILSpy). ILSply is an open source .NET
decompiler. It is normally invoked as a GUI application. However, its CLI
implementation ('ilspycmd') provides a better platform for automation.

## Features

- Recursive decompilation of .NET binaries in a given directory
- Duplicate .NET binary avoidance
- Configurable concurrent decompilation of .NET binaries

## Use cases
My primary use case for this application is to decompile all of the .NET files
in a given directory into a new directory whose structure mimics that of the
original directory.

I would like this to happen as quickly as possible too. That means the
application must support running multiple decompilers simultaneously. Lastly,
I would like to avoid decompiling the same .NET files over and over. Some
developers (who shall not be named) like to include the same .NET files
multiple times in their work.

All of these things are headaches that a simple script could never solve. That
is why finley exists :)

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
finley some-directory
```

If you would like to recursively decompile all of the binaries in a given
directory (i.e., include sub-directories), you can add the `-r` option:
```
finley -r some-directory
```

For additional options, run with the `-h` option. This will produce a list of
options and an explanation of their effects:
```
  -allow-duplicates
    	Decompile file even if its hash has already been encountered
  -e string
    	Comma separated list of file extensions to search for (default ".dll,.exe")
  -h	Display this help page
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
  -v	Display log messages rather than a progress bar
  -version
    	Display the version number and exit
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
