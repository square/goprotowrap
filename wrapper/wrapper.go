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

// Package wrapper implements the actual functionality of wrapping
// protoc.
package wrapper

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
)

// defaultProtocCommand is the default command used to call protoc.
const defaultProtocCommand = "protoc"

// The wrapper object.
type Wrapper struct {
	ProtocCommand string   // The command to call to run protoc.
	Parallelism   int      // Number of simultaneous calls to make to protoc when generating.
	ProtocFlags   []string // Flags to pass to protoc.
	ImportDirs    []string // Base directories in which .proto files reside.
	ProtoFiles    []string // The list of .proto files to generate code for.
	NoExpand      bool     // If true, don't search for other protos in import directories.
	PrintOnly     bool     // If true, don't generate: just print the protoc commandlines that would be called.

	allProtos   []string                // All proto files: those specified, plus those found alongside them.
	infos       map[string]*FileInfo    // A map of filename to FileInfo struct for all proto files we care about in this run.
	packages    map[string]*PackageInfo // A list of PackageInfo structs for packages containing files we care about.
	allPackages map[string]*PackageInfo // A list of PackageInfo structs for all packages.

	initCalled bool // Has Init() been called?

	// Used internally for checking for cycles and topologically sorting
	sccs [][]*PackageInfo // Slice of strongly-connected components in the package graph.
}

// Init must be called before any of the methods that do anything.
func (w *Wrapper) Init() error {
	if len(w.ImportDirs) == 0 {
		return errors.New("at least one import directory required")
	}
	for _, importDir := range w.ImportDirs {
		stat, err := os.Stat(importDir)
		if err != nil {
			return fmt.Errorf("Nonexistent import directory: %q", importDir)
		}
		if !stat.IsDir() {
			return fmt.Errorf("Non-directory import directory: %q", importDir)
		}
	}

	if len(w.ProtoFiles) == 0 {
		return errors.New("at least one input .proto file is required")
	}
	for _, file := range w.ProtoFiles {
		if !strings.HasSuffix(file, ".proto") {
			return fmt.Errorf("non-proto input file: %q", file)
		}
		if !w.inImportDir(file) {
			return fmt.Errorf("proto file %q must have a lexicographical prefix of one of the import directories", file)
		}
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return fmt.Errorf("input %q does not exist", file)
		}
	}

	if w.ProtocCommand == "" {
		w.ProtocCommand = defaultProtocCommand
	}

	// Get the list of actually-used import directories.
	dirs := w.importDirsUsed()

	expanded := []string{}
	// Unless asked not to, find all proto files with common import directory ancestors.
	if !w.NoExpand {
		neighbors, err := ProtosBelow(dirs)
		if err != nil {
			return err
		}
		expanded = Disjoint(w.ProtoFiles, neighbors)
	}

	w.allProtos = make([]string, len(w.ProtoFiles), len(w.ProtoFiles)+len(expanded))
	copy(w.allProtos, w.ProtoFiles)
	w.allProtos = append(w.allProtos, expanded...)
	var err error
	w.infos, err = GetFileInfos(w.ImportDirs, w.allProtos, w.ProtocCommand)
	if err != nil {
		return fmt.Errorf("cannot get .proto file information: %v", err)
	}

	AnnotateFullPaths(w.infos, w.allProtos, w.ImportDirs)
	ComputeGoLocations(w.infos)

	neededPackages := map[string]struct{}{}
	for _, proto := range w.ProtoFiles {
		info, ok := w.infos[FileDescriptorName(proto, w.ImportDirs)]
		if !ok {
			return fmt.Errorf("missing file info for %q.\n", proto)
		}
		neededPackages[info.ComputedPackage] = struct{}{}
	}

	w.allPackages, err = CollectPackages(w.infos, w.ProtoFiles, w.ImportDirs)
	if err != nil {
		return fmt.Errorf("cannot collect package information: %v", err)
	}

	w.packages = map[string]*PackageInfo{}
	for pkgName := range neededPackages {
		pkg, ok := w.allPackages[pkgName]
		if !ok {
			return fmt.Errorf("cannot find package information for %q", pkgName)
		}
		w.packages[pkgName] = pkg
	}

	w.initCalled = true
	return nil
}

// inImportDir returns true if the given file has a lexicographical
// prefix of one of the import directories.
func (w *Wrapper) inImportDir(file string) bool {
	for _, imp := range w.ImportDirs {
		if strings.HasPrefix(file, imp) {
			return true
		}
	}
	return false
}

// importDirsUsed returns the set of import directories that contain
// entries in the set of proto files.
func (w *Wrapper) importDirsUsed() []string {
	used := []string{}
	for _, imp := range w.ImportDirs {
		for _, proto := range w.ProtoFiles {
			if strings.HasPrefix(proto, imp) {
				used = append(used, imp)
				break
			}
		}
	}
	return used
}

// PrintStructure dumps out the computed structure to the given
// io.Writer.
func (w *Wrapper) PrintStructure(writer io.Writer) {
	if !w.initCalled {
		fmt.Fprintln(writer, "[Not initialized]")
		return
	}
	// Debugging output.
	fmt.Fprintln(writer, "> Structure:")
	for _, pkg := range w.packagesInOrder() {
		fmt.Fprintf(writer, "> %v\n", pkg.ComputedPackage)
		fmt.Fprintln(writer, ">   files:")
		for _, file := range pkg.Files {
			fmt.Fprintf(writer, ">     %v (%v)\n", file.Name, file.FullPath)
		}
		fmt.Fprintln(writer, ">   deps:")
		for _, file := range pkg.Deps {
			if file.FullPath != "" {
				fmt.Fprintf(writer, ">     %v (%v)\n", file.Name, file.FullPath)
			} else {
				fmt.Fprintf(writer, ">     %v\n", file.Name)
			}
		}
	}
}

// Generate actually generates the output files.
func (w *Wrapper) Generate() error {
	if !w.initCalled {
		return errors.New("Init() must be called before Generate()")
	}
	if w.Parallelism < 1 {
		return fmt.Errorf("parallelism cannot be < 1; got %d", w.Parallelism)
	}
	parallelism := len(w.packages)
	if w.Parallelism < parallelism {
		parallelism = w.Parallelism
	}

	pkgChan := make(chan *PackageInfo)

	errChan := make(chan error, parallelism)
	var wg sync.WaitGroup
	wg.Add(parallelism)
	for i := 0; i < parallelism; i++ {
		go func() {
			for pkg := range pkgChan {
				fmt.Printf("Generating package %s\n", pkg.ComputedPackage)
				if err := Generate(pkg, w.ImportDirs, w.ProtocCommand, w.ProtocFlags, w.PrintOnly); err != nil {
					errChan <- fmt.Errorf("error generating package %s: %v\n", pkg.ComputedPackage, err)
				}
			}
			wg.Done()
		}()
	}

	var err error
OUTER:
	for _, pkg := range w.packagesInOrder() {
		select {
		case pkgChan <- pkg:
		case err = <-errChan:
			break OUTER
		}
	}
	close(pkgChan)
	wg.Wait()
	return err
}

// packagesInOrder returns the list of packages, sorted by name.
func (w *Wrapper) packagesInOrder() []*PackageInfo {
	result := make([]*PackageInfo, 0, len(w.packages))
	names := make([]string, 0, len(w.packages))
	for name := range w.packages {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		result = append(result, w.packages[name])
	}
	return result
}

// allPackagesInOrder returns the list of all packages, sorted by name.
func (w *Wrapper) allPackagesInOrder() []*PackageInfo {
	result := make([]*PackageInfo, 0, len(w.allPackages))
	names := make([]string, 0, len(w.allPackages))
	for name := range w.allPackages {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		result = append(result, w.allPackages[name])
	}
	return result
}
