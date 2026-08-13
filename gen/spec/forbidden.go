package spec

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
)

// LoadForbidden reads an api-key-forbidden.txt data file and returns the set of
// Service.method routes that must be pruned from the generated surface.
//
// Documented format: one `Service.method` per line (exact case — method names
// are mixed case, e.g. Page.listPrefix); blank lines and `#` comment lines are
// ignored. For robustness the 403routes.txt capture format
// (`403  FORBIDDEN  <Service.method>  <message>`) is also accepted — the route
// is the third whitespace-separated field. Duplicates collapse into the set.
func LoadForbidden(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	set := make(map[string]bool)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		route := fields[0]
		if len(fields) >= 3 && isNumeric(fields[0]) {
			// Capture-format line: `403 FORBIDDEN Service.method message`.
			route = fields[2]
		}
		if route != "" {
			set[route] = true
		}
	}
	return set, sc.Err()
}

func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// PruneReport summarizes one Model.PruneForbidden call. It is printed by every
// generation so the numbers visibly drift when the spec or the forbidden list
// changes.
type PruneReport struct {
	Total           int      // methods in the model before pruning
	Pruned          int      // methods dropped (Service.method in the forbidden set)
	Kept            int      // methods that survive
	RemovedServices []string // services with zero methods after pruning
	Stale           []string // forbidden entries matching no method in the model
}

// PruneForbidden drops every method whose Service+"."+Method is in the
// forbidden set. Types are never pruned — types are not RPCs and cannot 403.
//
// Stale entries (forbidden routes that match no method in the spec) are
// returned in the report so the caller can warn loudly without failing the
// build — the forbidden list is a data snapshot and the live spec drifts.
func (m *Model) PruneForbidden(forbidden map[string]bool) PruneReport {
	report := PruneReport{Total: len(m.Methods)}

	known := make(map[string]bool)
	before := make(map[string]bool)
	for _, mi := range m.Methods {
		known[mi.Service+"."+mi.Method] = true
		before[mi.Service] = true
	}

	kept := m.Methods[:0]
	for _, mi := range m.Methods {
		if forbidden[mi.Service+"."+mi.Method] {
			continue
		}
		kept = append(kept, mi)
	}
	m.Methods = kept
	report.Pruned = report.Total - len(m.Methods)
	report.Kept = len(m.Methods)

	after := make(map[string]bool)
	for _, mi := range m.Methods {
		after[mi.Service] = true
	}
	for svc := range before {
		if !after[svc] {
			report.RemovedServices = append(report.RemovedServices, svc)
		}
	}
	sort.Strings(report.RemovedServices)

	for key := range forbidden {
		if !known[key] {
			report.Stale = append(report.Stale, key)
		}
	}
	sort.Strings(report.Stale)
	return report
}

// String renders the one-line prune summary printed by every generation.
func (r PruneReport) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "kept %d/%d methods (pruned %d)", r.Kept, r.Total, r.Pruned)
	if len(r.RemovedServices) > 0 {
		fmt.Fprintf(&b, "; %d services fully removed: %s", len(r.RemovedServices), strings.Join(r.RemovedServices, ", "))
	}
	return b.String()
}

// WarnStale logs the loud, non-fatal warning for forbidden entries that matched
// no method in the current spec. Expected-stale entries are normal (see the
// api-key-forbidden.txt header); anything beyond them means spec or list drift.
func WarnStale(report PruneReport) {
	if len(report.Stale) == 0 {
		return
	}
	log.Printf("WARNING: %d forbidden entries matched no method in the spec (stale entries — spec or list drift):", len(report.Stale))
	for _, key := range report.Stale {
		log.Printf("  stale: %s", key)
	}
}
