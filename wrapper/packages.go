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

// packages.go contains the code needed to figure out what packages we
// want .proto files to generate into. It calls out to protoc to parse
// the .proto files in to a FileDescriptorSet.

package wrapper

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
)

// FileInfo is the set of information we need to know about a file from protoc.
type FileInfo struct {
	Name      string   // The Name field of the FileDescriptorProto (import-path-relative)
	FullPath  string   // The full path to the file, as specified on the command-line
	Package   string   // The declared package
	GoPackage string   // The declared go_package
	Deps      []string // The names of files imported by this file (import-path-relative)

	// Our final decision for which package this file should generate
	// to. In the full form "path;decl" (whether decl is redundant or
	// not) as described in github.com/golang/protobuf/issues/139
	ComputedPackage string // Our final decision for which package this file should generate to
}

// PackageDir returns the desired directory location for the given
// file; ComputedPackage, with dots replaced by slashes.
func (f FileInfo) PackageDir() string {
	parts := strings.Split(f.ComputedPackage, ".")
	return filepath.Join(parts...)
}

// GoPluginOutputFilename returns the filename the vanilla go protoc
// plugin will use when generating output for this file.
func (f FileInfo) GoPluginOutputFilename() string {
	name := f.Name
	ext := path.Ext(name)
	if ext == ".proto" || ext == ".protodevel" {
		name = name[0 : len(name)-len(ext)]
	}
	return name + ".pb.go"
}

// PackageInfo collects all the information for a single package.
type PackageInfo struct {
	ComputedPackage string
	Files           []*FileInfo
	Deps            []*FileInfo

	// Internal use for cycle-checking and topological sorting.
	index   int
	lowlink int
	onStack bool
}

// PackageDir returns the desired directory location for the given
// package; ComputedPackage, with dots replaced by slashes.
func (p PackageInfo) PackageDir() string {
	parts := strings.Split(p.ComputedPackage, ".")
	return filepath.Join(parts...)
}

// PackageName returns the desired package name for the given package;
// whatever follows the last dot in ComputedPackage.
func (p PackageInfo) PackageName() string {
	parts := strings.Split(p.ComputedPackage, ".")
	return parts[len(parts)-1]
}

//ImportedPackageComputedNames returns the list of packages imported by this
//package.
func (p PackageInfo) ImportedPackageComputedNames() []string {
	result := []string{}
	seen := map[string]bool{
		p.ComputedPackage: true,
	}
	for _, d := range p.Deps {
		pkg := d.ComputedPackage
		if seen[pkg] {
			continue
		}
		seen[pkg] = true
		result = append(result, pkg)
	}
	return result
}

// GetFileInfos gets the FileInfo struct for every proto passed in.
func GetFileInfos(importPaths []string, protos []string, protocCommand string) (info map[string]*FileInfo, err error) {
	if len(importPaths) == 0 {
		return nil, fmt.Errorf("GetFileInfos: empty importPaths")
	}
	if len(protos) == 0 {
		return nil, fmt.Errorf("GetFileInfos: empty protos")
	}
	info = map[string]*FileInfo{}

	var dir string
	dir, err = ioutil.TempDir("", "filedescriptors")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err2 := os.RemoveAll(dir); err != nil && err2 != nil {
			err = err2
		}
	}()

	descriptorFilename := filepath.Join(dir, "all.pb")

	args := []string{}
	for _, importPath := range importPaths {
		args = append(args, "-I", importPath)
	}
	args = append(args, "--descriptor_set_out="+descriptorFilename)

	args = append(args, "--include_imports")

	for _, proto := range protos {
		args = append(args, proto)
	}

	fmt.Println("Collecting filedescriptors...")
	cmd := exec.Command(protocCommand, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		cmdline := fmt.Sprintf("%s %s\n", protocCommand, strings.Join(args, " "))
		return nil, fmt.Errorf("error running %v\n%v\nOutput:\n======\n%s======\n", cmdline, err, out)
	}
	descriptorSetBytes, err := ioutil.ReadFile(descriptorFilename)
	if err != nil {
		return nil, err
	}

	descriptorSet := &descriptor.FileDescriptorSet{}
	err = proto.Unmarshal(descriptorSetBytes, descriptorSet)
	if err != nil {
		return nil, err
	}

	for _, fd := range descriptorSet.File {
		fi := &FileInfo{
			Name:    fd.GetName(),
			Package: fd.GetPackage(),
		}
		for _, dep := range fd.Dependency {
			fi.Deps = append(fi.Deps, dep)
		}
		fi.GoPackage = fd.Options.GetGoPackage()
		info[fi.Name] = fi
	}

	return info, nil
}

// ComputeGoLocations uses the package and go_package information to
// figure out the effective Go location and package.  It sets
// ComputedPackage to the full form "path;decl" (whether decl is
// redundant or not) as described in
// github.com/golang/protobuf/issues/139
func ComputeGoLocations(infos map[string]*FileInfo) {
	for _, info := range infos {
		dir := filepath.Dir(info.Name)
		pkg := info.GoPackage
		// The presence of a slash implies there's an import path.
		slash := strings.LastIndex(pkg, "/")
		if slash > 0 {
			if strings.Contains(pkg, ";") {
				info.ComputedPackage = pkg
				continue
			}
			decl := pkg[slash+1:]
			info.ComputedPackage = pkg + ";" + decl
			continue
		}
		if pkg == "" {
			pkg = info.Package
		}
		if pkg == "" {
			pkg = baseName(info.Name)
			fmt.Fprintf(os.Stderr, "Warning: file %q has no go_package and no package.\n", info.Name)
		}
		info.ComputedPackage = dir + ";" + strings.Map(badToUnderscore, pkg)
	}
}

// baseName returns the last path element of the name, with the last dotted suffix removed.
func baseName(name string) string {
	// First, find the last element
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	// Now drop the suffix
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[0:i]
	}
	return name
}

// CollectPackages returns a map of PackageInfos.
func CollectPackages(infos map[string]*FileInfo, protos []string, importDirs []string) (map[string]*PackageInfo, error) {
	pkgMap := map[string]*PackageInfo{}

	// Collect into packages.
	for _, info := range infos {
		packageInfo, ok := pkgMap[info.ComputedPackage]
		if !ok {
			packageInfo = &PackageInfo{ComputedPackage: info.ComputedPackage}
			pkgMap[info.ComputedPackage] = packageInfo
		}
		packageInfo.Files = append(packageInfo.Files, info)
	}

	// Collect deps for each package.
	for _, pkg := range pkgMap {
		deps := map[string]*FileInfo{}
		for _, info := range pkg.Files {
			for _, dep := range info.Deps {
				deps[dep] = infos[dep]
			}
		}
		for _, dep := range deps {
			pkg.Deps = append(pkg.Deps, dep)
		}
	}

	return pkgMap, nil
}

// FileDescriptorName computes the import-dir-relative Name that the
// FileDescriptor for a full filename will have.
func FileDescriptorName(protoFile string, importDirs []string) string {
	isAbs := path.IsAbs(protoFile)
	for _, imp := range importDirs {
		// Handle import dirs of "." - the FileDescriptorProtos don't have the "./" prefix.
		if imp == "." && !isAbs {
			if strings.HasPrefix(protoFile, "./") {
				return protoFile[2:]
			}
			return protoFile
		}
		if strings.HasPrefix(protoFile, imp) {
			name := protoFile[len(imp):]
			if strings.HasPrefix(name, "/") && imp != "/" {
				return name[1:]
			}
			return name
		}
	}
	panic(fmt.Sprintf("Unable to find import dir for %q", protoFile))
}

// AnnotateFullPaths annotates an existing set of FileInfos with their
// full paths.
func AnnotateFullPaths(infos map[string]*FileInfo, allProtos []string, importDirs []string) {
	for _, proto := range allProtos {
		name := FileDescriptorName(proto, importDirs)
		info, ok := infos[name]
		if !ok {
			panic(fmt.Sprintf("Unable to find file information for %q", name))
		}
		info.FullPath = proto
	}
}

// badToUnderscore is the mapping function used to generate Go names from package names,
// which can be dotted in the input .proto file.  It replaces non-identifier characters such as
// dot or dash with underscore.
func badToUnderscore(r rune) rune {
	if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
		return r
	}
	return '_'
}
