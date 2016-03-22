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

// File flags.go contains code to parse protoc-style commandline
// arguments.

package wrapper

import (
	"fmt"
	"strconv"
	"strings"
)

// Flags that take no values. See
// https://github.com/google/protobuf/blob/5e933847/src/google/protobuf/compiler/command_line_interface.cc#L1069
var noValueFlags = map[string]bool{
	"-h":                         true,
	"--help":                     true,
	"--disallow_services":        true,
	"--include_imports":          true,
	"--include_source_info":      true,
	"--version":                  true,
	"--decode_raw":               true,
	"--print_free_field_numbers": true,
}

// Flag values is a simple map of parsed flag values. A map of string
// to string, with convenience getters.
type FlagValues map[string]string

// ParseArgs parses protoc-style commandline arguments, splitting them
// into custom flags, protoc flags and input files, and capturing a
// list of import directories. Custom flag names are passed without
// dashes, and are expected to be specified with two dashes. If
// customFlagNames[name] is true, the custom flag expects a value;
// otherwise it can have no value, and will get a value of "".
func ParseArgs(args []string, custom map[string]bool) (customFlags FlagValues, protocFlags, protos, importDirs []string, err error) {
	customFlags = make(FlagValues)

	var nextIsFlag, nextIsCustomFlag, nextIsImportDir bool
	var customName string
	for _, arg := range args {
		// Catch empty, "-" and "--" arguments. See
		// https://github.com/google/protobuf/blob/5e933847/src/google/protobuf/compiler/command_line_interface.cc#L1049
		if arg == "" || arg == "-" || arg == "--" {
			return nil, nil, nil, nil, fmt.Errorf("flag %q not allowed", arg)
		}

		if nextIsCustomFlag {
			customFlags[customName] = arg
			nextIsCustomFlag = false
			continue
		}
		if nextIsFlag {
			protocFlags = append(protocFlags, arg)
			nextIsFlag = false
			if nextIsImportDir {
				importDirs = append(importDirs, arg)
				nextIsImportDir = false
			}
			continue
		}
		if noValueFlags[arg] {
			protocFlags = append(protocFlags, arg)
			continue
		}
		if arg[0] != '-' {
			protos = append(protos, arg)
			continue
		}

		// Two dashes. Expect "=" or second arg for value.
		if arg[1] == '-' {
			parts := strings.SplitN(arg[2:], "=", 2)
			name := parts[0]
			needsValue, isCustom := custom[name]
			if isCustom {
				if len(parts) == 1 {
					if needsValue {
						nextIsCustomFlag = true
						customName = name
					} else {
						customFlags[name] = ""
					}
				} else {
					customFlags[name] = parts[1]
				}
				continue
			}
			protocFlags = append(protocFlags, arg)
			if len(parts) == 1 {
				nextIsFlag = true
			}
			continue
		}

		protocFlags = append(protocFlags, arg)
		// One dash. Expect single-char flag with value concatenated, or second arg for value.
		// Capture import directory (-I) values separately.
		if len(arg) == 2 {
			nextIsFlag = true
			nextIsImportDir = arg[1] == 'I'
			continue
		}
		if arg[1] == 'I' {
			importDirs = append(importDirs, arg[2:])
		}
	}
	if nextIsFlag || nextIsCustomFlag {
		return nil, nil, nil, nil, fmt.Errorf("%q flag with no value", args[len(args)-1])
	}
	return customFlags, protocFlags, protos, importDirs, nil
}

// Int returns the integer version of a flag, if set.
func (fv FlagValues) Int(name string, defaultValue int) (int, error) {
	value, found := fv[name]
	if !found {
		return defaultValue, nil
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("flag %q: cannot parse integer from %q", name, value)
	}
	return i, nil
}

// Bool returns the boolean version of a flag, if set.
func (fv FlagValues) Bool(name string, defaultValue bool) (bool, error) {
	value, found := fv[name]
	if !found {
		return defaultValue, nil
	}
	switch value {
	case "", "t", "T", "true", "True", "1":
		return true, nil
	case "f", "F", "false", "False", "0":
		return false, nil
	}
	return false, fmt.Errorf("flag %q: cannot parse boolean from %q", name, value)
}

// String returns the string version of a flag, if set.
func (fv FlagValues) String(name string, defaultValue string) string {
	value, found := fv[name]
	if !found {
		return defaultValue
	}
	return value
}
