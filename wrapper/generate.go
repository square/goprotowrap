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

// generate.go contains the code that does the actual generation.

package wrapper

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// Generate does the actual generation of protos.
func Generate(pkg *PackageInfo, importDirs []string, protocCommand string, protocFlags []string, printOnly bool) (err error) {
	args := protocFlags[0:len(protocFlags):len(protocFlags)]

	files := make([]string, 0, len(pkg.Files))
	for _, f := range pkg.Files {
		files = append(files, f.FullPath)
	}
	sort.Strings(files)
	args = append(args, files...)

	if printOnly {
		fmt.Printf("%s %s\n", protocCommand, strings.Join(args, " "))
		return nil
	}
	cmd := exec.Command(protocCommand, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		cmdline := fmt.Sprintf("%s %s\n", protocCommand, strings.Join(args, " "))
		return fmt.Errorf("error running %v\n%v\nOutput:\n======\n%s======\n", cmdline, err, out)
	}
	return nil
}

var packageRe = regexp.MustCompile(`^package [\p{L}_][\p{L}\p{N}_]*`)

// CopyAndChangePackage copies file `in` to file `out`, rewriting the
// `package` declaration to `pkg`.
func CopyAndChangePackage(in, out, pkg string) error {
	inf, err := os.Open(in)
	if err != nil {
		return err
	}
	defer inf.Close()
	outf, err := os.Create(out)
	if err != nil {
		return err
	}
	defer outf.Close()
	scanner := bufio.NewScanner(inf)
	matched := false
	for scanner.Scan() {
		line := scanner.Text()
		if !matched && packageRe.MatchString(line) {
			matched = true
			line = packageRe.ReplaceAllString(line, fmt.Sprintf("package %s", pkg))
		}
		fmt.Fprintln(outf, line)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}
