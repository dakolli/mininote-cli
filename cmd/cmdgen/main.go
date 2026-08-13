// Command cmdgen generates the cobra command tree for the mininote CLI.
//
// It reads an intro.json spec and renders cmd/commands.gen.go (package cmd),
// one service command per RPC service and one subcommand per method. The
// generated file is the only thing wired into the hand-written CLI: it
// exposes registerServiceCommands(root, getClient) which the root command
// calls from its init.
package main

import (
	"bytes"
	"context"
	_ "embed"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"

	"github.com/dakolli/mininote-cli/gen/spec"
)

//go:embed commands.tmpl
var templateSrc string

const (
	forbiddenName = "api-key-forbidden.txt"
	defaultOut    = "cmd/commands.gen.go"
)

func main() {
	out := flag.String("out", defaultOut, "output file for generated cobra commands")
	introspect := flag.String("introspect", spec.DefaultIntrospectURL, "live spec capture URL; the captured spec is written to <repo root>/intro.json on every run")
	forbidden := flag.String("forbidden", "", "path to api-key-forbidden.txt (default: <repo root>/api-key-forbidden.txt)")
	full := flag.Bool("full", false, "skip forbidden-list pruning and generate the complete surface from the spec")
	flag.Parse()

	// The documented defaults are relative to cmd/ (where go:generate runs);
	// recompute them from the module root so the generator also works when
	// invoked manually from anywhere inside the module.
	root, err := moduleRoot()
	if err != nil {
		log.Fatalf("find module root: %v", err)
	}
	in := filepath.Join(specDir(root), "intro.json")
	if !flagSet("out") {
		*out = filepath.Join(root, "cmd", "commands.gen.go")
	}
	if *forbidden == "" {
		*forbidden = filepath.Join(specDir(root), forbiddenName)
	}

	// The spec is always re-captured live before generation: intro.json is a
	// committed snapshot of the last live capture, never an offline fallback.
	if _, _, err := spec.CaptureSpec(context.Background(), *introspect, spec.SpecToken(), in); err != nil {
		log.Fatalf("capture spec from %s: %v (set MININOTE_ADMIN_AGENT_KEY or MININOTE_TOKEN)", *introspect, err)
	}

	s, err := spec.LoadSpec(in)
	if err != nil {
		log.Fatalf("load spec: %v", err)
	}
	model, err := spec.Normalize(s)
	if err != nil {
		log.Fatalf("normalize spec: %v", err)
	}

	if *full {
		log.Printf("full surface requested (-full): skipping forbidden-list pruning")
	} else {
		forbiddenSet, err := spec.LoadForbidden(*forbidden)
		if err != nil {
			log.Printf("WARNING: cannot load forbidden list %s: %v — generating FULL surface (pass -full to silence)", *forbidden, err)
		} else {
			report := model.PruneForbidden(forbiddenSet)
			log.Printf("forbidden list: %s", *forbidden)
			log.Printf("prune summary: %s", report.String())
			spec.WarnStale(report)
		}
	}

	svcs, methods := buildServices(s, model)
	hasJSON := false
	for _, svc := range svcs {
		for _, m := range svc.Methods {
			for _, p := range m.Params {
				if p.Kind == "json" {
					hasJSON = true
				}
			}
		}
	}

	tmpl, err := template.New("commands").Funcs(template.FuncMap{
		"varName":     varName,
		"flagSetFn":   flagSetFn,
		"flagDefault": flagDefault,
		"varType":     varType,
	}).Parse(templateSrc)
	if err != nil {
		log.Fatalf("parse template: %v", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct {
		Services   []serviceView
		JSONParams bool
	}{svcs, hasJSON}); err != nil {
		log.Fatalf("execute template: %v", err)
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		log.Fatalf("gofmt generated commands: %v\n--- raw output ---\n%s", err, buf.Bytes())
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", filepath.Dir(*out), err)
	}
	if err := os.WriteFile(*out, src, 0o644); err != nil {
		log.Fatalf("write %s: %v", *out, err)
	}
	log.Printf("generated %d service commands with %d method commands -> %s", len(svcs), methods, *out)
}

// flagSet reports whether the named flag was given on the command line.
func flagSet(name string) bool {
	set := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

// moduleRoot walks up from the working directory to the dir containing go.mod.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found above %s", dir)
		}
		dir = parent
	}
}

// specDir returns the directory holding the spec (intro.json).
// The module root IS the repo root.
func specDir(root string) string {
	return root
}

// serviceView is one RPC service rendered as a cobra command.
type serviceView struct {
	Use     string
	Short   string
	Methods []methodView
}

// methodView is one RPC method rendered as a cobra subcommand.
type methodView struct {
	CmdFunc     string
	Method      string
	Short       string
	GoName      string
	HasParams   bool
	RequestType string
	Params      []paramView
}

// paramView is one request field rendered as a command-line flag.
type paramView struct {
	JsonName string
	GoName   string
	GoType   string
	Required bool
	Optional bool // *string/*float64/*bool: only set when the flag is passed
	Kind     string
	Help     string
}

// buildServices groups normalized methods by service, preserving the model's
// sorted service order, and attaches each service's title from the spec.
func buildServices(s *spec.Spec, model *spec.Model) ([]serviceView, int) {
	var svcs []serviceView
	byName := make(map[string]int)
	total := 0
	for _, m := range model.Methods {
		idx, ok := byName[m.Service]
		if !ok {
			title := m.Service
			if descs := s.Services[m.Service]; len(descs) > 0 && descs[0].Title != "" {
				title = descs[0].Title
			}
			svcs = append(svcs, serviceView{
				Use:   lowerFirst(m.Service),
				Short: title + " service",
			})
			idx = len(svcs) - 1
			byName[m.Service] = idx
		}
		mv := methodView{
			CmdFunc:     "cmd" + m.GoName,
			Method:      m.Method,
			Short:       m.Title,
			GoName:      m.GoName,
			HasParams:   m.HasParams,
			RequestType: m.RequestType,
		}
		for _, f := range m.Params {
			mv.Params = append(mv.Params, paramView{
				JsonName: f.JsonName,
				GoName:   f.GoName,
				GoType:   f.GoType,
				Required: f.Required,
				Optional: strings.HasPrefix(f.GoType, "*"),
				Kind:     paramKind(f.GoType),
				Help:     paramHelp(f),
			})
		}
		svcs[idx].Methods = append(svcs[idx].Methods, mv)
		total++
	}
	return svcs, total
}

// paramKind classifies a Go field type into the flag family used for it.
func paramKind(gt string) string {
	switch gt {
	case "string", "*string":
		return "string"
	case "float64", "*float64":
		return "float64"
	case "bool", "*bool":
		return "bool"
	case "[]string":
		return "stringSlice"
	case "[]float64":
		return "float64Slice"
	case "[]bool":
		return "boolSlice"
	}
	return "json"
}

// paramHelp composes the flag help: the field doc, a JSON marker for
// JSON-string flags, and the example when the spec provides one.
func paramHelp(f spec.FieldInfo) string {
	var parts []string
	if f.Doc != "" {
		parts = append(parts, f.Doc)
	}
	if paramKind(f.GoType) == "json" {
		parts = append(parts, "JSON")
	}
	if f.Example != "" {
		parts = append(parts, "example: "+f.Example)
	}
	return strings.Join(parts, "; ")
}

// lowerFirst lower-cases the first rune of s (service name -> command name).
func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func varName(goName string) string { return "p" + goName }

func varType(goType string) string {
	switch paramKind(goType) {
	case "string", "json":
		return "string"
	case "float64":
		return "float64"
	case "bool":
		return "bool"
	case "stringSlice":
		return "[]string"
	case "float64Slice":
		return "[]float64"
	case "boolSlice":
		return "[]bool"
	}
	return "string"
}

// flagSetFn is the pflag Var method suffix for a flag family.
func flagSetFn(kind string) string {
	switch kind {
	case "float64":
		return "Float64"
	case "bool":
		return "Bool"
	case "stringSlice":
		return "StringSlice"
	case "float64Slice":
		return "Float64Slice"
	case "boolSlice":
		return "BoolSlice"
	}
	return "String"
}

// flagDefault is the pflag default value literal for a flag family.
func flagDefault(kind string) string {
	switch kind {
	case "float64":
		return "0"
	case "bool":
		return "false"
	case "stringSlice", "float64Slice", "boolSlice":
		return "nil"
	}
	return `""`
}
