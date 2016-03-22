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

// finding.go contains the functions responsible for the building the
// initial list of protobufs to build, and the list of import path
// directories that contain them.

package wrapper

import (
	"os"
	"path/filepath"
	"strings"
)

// ProtosBelow returns a slice containing the filenames of all .proto
// files found in or below the given directories.
func ProtosBelow(dirs []string) ([]string, error) {
	protos := []string{}
	for _, dir := range dirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".proto") {
				protos = append(protos, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return protos, nil
}

// ImportDirsUsed returns the set of import directories that contain
// entries in the set of proto files.
func ImportDirsUsed(importDirs []string, protos []string) []string {
	used := []string{}
	for _, imp := range importDirs {
		for _, proto := range protos {
			if strings.HasPrefix(proto, imp) {
				used = append(used, imp)
				break
			}
		}
	}
	return used
}

// Disjoint takes a slice of existing .proto files, and a slice of new
// .proto files. It returns a slice containing the subset of the new
// .proto files with distinct paths not in the first set.
func Disjoint(existing, additional []string) []string {
	set := make(map[string]bool, len(existing)+len(additional))
	result := make([]string, 0, len(additional))
	for _, proto := range existing {
		set[proto] = true
	}

	for _, proto := range additional {
		if set[proto] {
			continue
		}
		set[proto] = true
		result = append(result, proto)
	}
	return result
}
