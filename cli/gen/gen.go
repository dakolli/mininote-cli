package main

import (
	"bytes"
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
	defaultIn = "../intro.json"
	outTypes  = "types.gen.go"
	outMethod = "methods.gen.go"
)

func main() {
	in := flag.String("in", defaultIn, "path to the intro.json spec file")
	typesOut := flag.String("types", "", fmt.Sprintf("output file for generated types (default: <module root>/client/%s)", outTypes))
	methodsOut := flag.String("methods", "", fmt.Sprintf("output file for generated methods (default: <module root>/client/%s)", outMethod))
	flag.Parse()

	s, err := spec.LoadSpec(*in)
	if err != nil {
		log.Fatalf("load spec: %v", err)
	}
	model, err := spec.Normalize(s)
	if err != nil {
		log.Fatalf("normalize spec: %v", err)
	}

	root, err := moduleRoot()
	if err != nil {
		log.Fatalf("find module root: %v", err)
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
