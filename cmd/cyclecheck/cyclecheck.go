// Copyright 2016 Square, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Binary cyclecheck looks for situations where imports in .proto
// files would result in circular package dependencies in the
// generated Go code.
package main

import (
	"fmt"
	"os"

	"github.com/square/goprotowrap/wrapper"
)

// customFlags is a map describing flags we add to protoc. true means
// a value is required. false implies boolean.
var customFlags = map[string]bool{
	"print_structure":      false,
	"protoc_command":       true,
	"only_specified_files": false,
}

func usageAndExit(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintf(os.Stderr, "Usage: %s [flags] [protofiles]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, `  --only_specified_files true|false
      if true, don't search the nearest import path ancestor for other .proto files
  --protoc_command string
      command to use to call protoc (default "protoc")
  --print_structure
      if true, print out computed package structure
`)
	os.Exit(1)
}

func main() {
	flags, protocFlags, protos, importDirs, err := wrapper.ParseArgs(os.Args[1:], customFlags)
	if err != nil {
		usageAndExit("Error: %v\n", err)
	}
	if len(importDirs) == 0 {
		usageAndExit("Error: at least one import directory (-I) needed\n")
	}

	noExpand, err := flags.Bool("only_specified_files", false)
	if err != nil {
		usageAndExit("Error: %v\n", err)
	}
	printStructure, err := flags.Bool("print_structure", false)
	if err != nil {
		usageAndExit("Error: %v\n", err)
	}

	w := &wrapper.Wrapper{
		ProtocCommand: flags.String("protoc_command", "protoc"),
		ProtocFlags:   protocFlags,
		ProtoFiles:    protos,
		ImportDirs:    importDirs,
		NoExpand:      noExpand,
	}
	err = w.Init()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Debugging output.
	if printStructure {
		w.PrintStructure(os.Stdout)
	}

	if err := w.CheckCycles(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v", err)
		os.Exit(1)
	}
}
