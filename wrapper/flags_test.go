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

package wrapper

import (
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	// func ParseArgs(args []string, custom map[string]bool) (customFlags map[string]string, protocFlags, protos, importDirs []string, err error) {

	realCustom := map[string]bool{
		"parallelism":          true,
		"print_structure":      false,
		"protoc_command":       true,
		"only_specified_files": false,
	}

	tests := map[string]struct {
		args        string
		custom      map[string]bool
		customFlags map[string]string
		protocFlags []string
		protos      []string
		importDirs  []string
		err         bool
	}{
		"basic": {
			"-I. foo1.proto -I includes foo2.proto --go-square_out=plugins=sake+grpc,import_prefix=foo:output/protos",
			realCustom,
			nil,
			[]string{"-I.", "-I", "includes", "--go-square_out=plugins=sake+grpc,import_prefix=foo:output/protos"},
			[]string{"foo1.proto", "foo2.proto"},
			[]string{".", "includes"},
			false,
		},
		"real custom args": {
			"-I. foo1.proto --parallelism=3 --only_specified_files=false --print_structure --protoc_command protoc",
			realCustom,
			map[string]string{
				"parallelism":          "3",
				"only_specified_files": "false",
				"print_structure":      "",
				"protoc_command":       "protoc",
			},
			[]string{"-I."},
			[]string{"foo1.proto"},
			[]string{"."},
			false,
		},
		"missing flag value": {
			"-I. foo1.proto --parallelism=3 --only_specified_files=false --print_structure --protoc_command",
			realCustom,
			nil,
			nil,
			nil,
			nil,
			true,
		},
	}

	for name, tt := range tests {
		if name[0] == '_' {
			continue
		}
		cf, pf, p, id, err := ParseArgs(strings.Split(tt.args, " "), tt.custom)
		if tt.err {
			if err == nil {
				t.Errorf("%q: want error; got nil", name)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error: %v", name, err)
			continue
		}

		if (tt.err && err == nil) || (!tt.err && err != nil) {
			t.Errorf("%q: want error=%v; got %v", name, tt.err, err)
		}
		if !mapStringStringEqual(cf, tt.customFlags) {
			t.Errorf("%q: want customFlags=%v; got %v", name, tt.customFlags, cf)
		}
		if !sliceStringEqual(pf, tt.protocFlags) {
			t.Errorf("%q: want protocFlags=%v; got %v", name, tt.protocFlags, pf)
		}
		if !sliceStringEqual(p, tt.protos) {
			t.Errorf("%q: want protos=%v; got %v", name, tt.protos, p)
		}
		if !sliceStringEqual(id, tt.importDirs) {
			t.Errorf("%q: want importDirs=%v; got %v", name, tt.importDirs, id)
		}
	}
}
