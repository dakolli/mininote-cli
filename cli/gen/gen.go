package main

import (
	"bytes"
	"context"
	"embed"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"mininote.dev/cli/gen/spec"
)

//go:embed templates/*.tmpl
var templates embed.FS

const (
	defaultIn      = "../intro.json"
	introspectName = "intro.json"
	forbiddenName  = "api-key-forbidden.txt"
	outTypes       = "types.gen.go"
	outMethod      = "methods.gen.go"
)

func main() {
	in := flag.String("in", defaultIn, "path to the intro.json spec file (passing it explicitly skips the live capture and uses the file as-is — the committed spec is a STALE offline fallback)")
	introspect := flag.String("introspect", spec.DefaultIntrospectURL, "live spec capture URL; the captured spec is written to <repo root>/intro.json on every run")
	forbidden := flag.String("forbidden", "", "path to api-key-forbidden.txt (default: <repo root>/api-key-forbidden.txt)")
	full := flag.Bool("full", false, "skip forbidden-list pruning and generate the complete surface from the spec")
	typesOut := flag.String("types", "", fmt.Sprintf("output file for generated types (default: <module root>/client/%s)", outTypes))
	methodsOut := flag.String("methods", "", fmt.Sprintf("output file for generated methods (default: <module root>/client/%s)", outMethod))
	flag.Parse()

	root, err := moduleRoot()
	if err != nil {
		log.Fatalf("find module root: %v", err)
	}
	if *forbidden == "" {
		*forbidden = filepath.Join(filepath.Dir(root), forbiddenName)
	}

	// The spec is always re-captured live unless -in was passed explicitly: the
	// committed intro.json is a stale fallback for offline builds only.
	if flagSet("in") {
		spec.StaleSpecWarning(*in)
	} else {
		specPath := filepath.Join(filepath.Dir(root), introspectName)
		if _, _, err := spec.CaptureSpec(context.Background(), *introspect, spec.SpecToken(), specPath); err != nil {
			log.Fatalf("capture spec from %s: %v (set MININOTE_ADMIN_AGENT_KEY or MININOTE_TOKEN)", *introspect, err)
		}
		*in = specPath
	}

	s, err := spec.LoadSpec(*in)
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

	if *typesOut == "" {
		*typesOut = filepath.Join(root, "client", outTypes)
	}
	if *methodsOut == "" {
		*methodsOut = filepath.Join(root, "client", outMethod)
	}

	if err := render("types.gen.tmpl", *typesOut, struct {
		Types []spec.TypeInfo
	}{model.Types}); err != nil {
		log.Fatalf("render types: %v", err)
	}
	if err := render("methods.gen.tmpl", *methodsOut, struct {
		Methods []spec.MethodInfo
	}{model.Methods}); err != nil {
		log.Fatalf("render methods: %v", err)
	}

	log.Printf("generated %d types -> %s", len(model.Types), *typesOut)
	log.Printf("generated %d methods -> %s", len(model.Methods), *methodsOut)
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

// render executes the named template and writes gofmt-formatted output.
func render(tmplName, outPath string, data any) error {
	funcs := template.FuncMap{
		"methodDoc": methodDoc,
	}
	tmpl, err := template.New("gen").Funcs(funcs).ParseFS(templates, "templates/*.tmpl")
	if err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}
	t := tmpl.Lookup(tmplName)
	if t == nil {
		return fmt.Errorf("template %q not found", tmplName)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute %s: %w", tmplName, err)
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("gofmt output for %s: %w", outPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outPath, src, 0o644)
}

// methodDoc builds the one-line doc comment for a generated client method.
func methodDoc(m spec.MethodInfo) string {
	return fmt.Sprintf("%s calls the %s.%s RPC.", m.GoName, m.Service, m.Method)
}
