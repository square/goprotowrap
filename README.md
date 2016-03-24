# goprotowrap

A package-at-a-time wrapper for protoc, for generating Go protobuf
code.

[![Build Status](https://travis-ci.org/square/goprotowrap.svg?branch=master)](https://travis-ci.org/square/goprotowrap)

`protowrap` is a small tool we found helpful when working with
[protocol buffers](https://developers.google.com/protocol-buffers/) in
Go at Square. We're publishing it in the hope that others find it
useful too. Contributions are welcome.

## Install

```shell
go get -u github.com/square/goprotowrap/cmd/protowrap
```

## Philosophy

Unlike other language plugins, the Go
[protobuf plugin](https://github.com/golang/protobuf) expects to be
called separately for each package, and given all files in that
package.

`protowrap` is called instead of `protoc`, and ensures that `.proto`
files are processed one Go package at a time.

It parses out the flags it understands, passing the rest through to
`protoc` unchanged.

## Operation

- search for all `.proto` files under the same import paths as the
  `.proto` file arguments
- call `protoc` to generate FileDescriptorProtos
- inspect the FileDescriptorProtos to deduce package information
- group `.proto` files into packages
- call `protoc` once for each package

## TODOs

- [x] (Soon) Replace square-specific handling of `go_package` with
      recently updated upstream logic.
- [ ] In the initial call to `protoc` for generating
      FileDescriptorProtos, pass `.proto` files to `protoc` in batches
      instead of all at once.
- [ ] Better tests, especially of the code paths not exercised by our
      build process.

## License

Copyright 2016 Square, Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you
may not use this file except in compliance with the License. You may
obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
implied. See the License for the specific language governing
permissions and limitations under the License.


