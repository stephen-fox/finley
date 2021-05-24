// Package finley makes it easy to quickly decompile several .NET binaries
// concurrently without involving a GUI.
//
// It does this by extending the CLI implementation of ILSpy
// (https://github.com/icsharpcode/ILSpy). ILSply is an open source .NET
// decompiler. It is normally invoked as a GUI application. However, its CLI
// implementation ('ilspycmd') provides a better platform for automation.
//
// The primary features of finley include recursive decompilation of .NET
// binaries in a given directory, duplicate .NET binary avoidance, and
// configurable concurrent decompilation of .NET binaries.
package finley
