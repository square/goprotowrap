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
	"fmt"
	"strings"
)

// CheckCycles checks for proto import structures that would result in
// Go package cycles.
func (w *Wrapper) CheckCycles() error {
	w.sccs = w.tarjan()
	cycles := []string{}
	for _, scc := range w.sccs {
		if len(scc) > 1 {
			cycles = append(cycles, w.showComponent(scc))
		}
	}
	if len(cycles) > 0 {
		return fmt.Errorf("cycles found:\n%s\n", strings.Join(cycles, "\n"))
	}
	return nil
}

// https://en.wikipedia.org/wiki/Tarjan%27s_SCC_algorithm
func (wr *Wrapper) tarjan() [][]*PackageInfo {
	index := 1
	sccs := [][]*PackageInfo{}
	s := []*PackageInfo{}

	var strongConnect func(*PackageInfo)
	strongConnect = func(v *PackageInfo) {
		// Set the depth index for v to the smallest unused index.
		v.index = index
		v.lowlink = index
		index++
		s = append(s, v)
		v.onStack = true

		// Consider successors of v.
		for _, wName := range v.ImportedPackageComputedNames() {
			w, ok := wr.allPackages[wName]
			if !ok {
				panic(fmt.Sprintf("%q not found in %v", wName, wr.allPackages))
			}
			if w.index == 0 {
				// Successor w has not yet been visited; recurse on it
				strongConnect(w)
				if w.lowlink < v.lowlink {
					v.lowlink = w.lowlink
				}
			} else {
				if w.onStack {
					// Successor w is in stack s and hence in the current SCC
					if w.index < v.lowlink {
						v.lowlink = w.index
					}
				}
			}
		}

		// If v is a root node, pop the stack and generate an SCC
		if v.lowlink == v.index {
			scc := []*PackageInfo{}
			var w *PackageInfo
			for w != v {
				w = s[len(s)-1]
				s = s[:len(s)-1]
				w.onStack = false
				scc = append(scc, w)
			}
			sccs = append(sccs, scc)
		}
	}

	for _, pkg := range wr.packagesInOrder() {
		if pkg.index == 0 {
			strongConnect(pkg)
		}
	}

	return sccs
}

// showComponent returns a string describing why a strongly connected
// component is strongly connected.
func (w *Wrapper) showComponent(pkgs []*PackageInfo) string {
	result := []string{}
	inCycle := map[string]bool{}
	for _, pkg := range pkgs {
		inCycle[pkg.ComputedPackage] = true
	}
	for _, pkg := range pkgs {
		for _, other := range pkg.ImportedPackageComputedNames() {
			if inCycle[other] {
				result = append(result, fmt.Sprintf(" %s --> %s", pkg.ComputedPackage, other))
				for _, f := range pkg.Files {
					for _, depName := range f.Deps {
						dep := w.infos[depName]
						if dep.ComputedPackage == other {
							result = append(result, fmt.Sprintf("  %s imports %s", f.Name, dep.Name))
						}
					}
				}
			}
		}
	}
	return strings.Join(result, "\n")
}
