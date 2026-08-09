// Package spec loads and normalizes a mininote intro.json spec into a
// Go-ready model shared by the code generators (client types/methods and the
// cobra command tree).
package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"
)

// Spec mirrors the top-level intro.json structure.
type Spec struct {
	Services map[string][]ServiceDesc `json:"services"`
	Types    map[string]*TypeDef      `json:"types"`
}

// ServiceDesc is one service descriptor. Each service has exactly one.
type ServiceDesc struct {
	Router  string   `json:"router"`
	Title   string   `json:"title"`
	Methods []Method `json:"methods"`
}

// Method is one RPC on a service.
type Method struct {
	Method             string             `json:"method"`
	Title              string             `json:"title"`
	PostPath           string             `json:"postPath"`
	HasParams          bool               `json:"hasParams"`
	Params             []Field            `json:"params"`
	RequestTypeScript  string             `json:"requestTypeScript"`
	ResponseTypeScript string             `json:"responseTypeScript"`
	ResponseTypeName   string             `json:"responseTypeName"`
	NestedTypes        map[string][]Field `json:"nestedTypes"`
}

// TypeDef is one named type.
type TypeDef struct {
	Name   string  `json:"name"`
	Fields []Field `json:"fields"`
	UsedBy []Usage `json:"used_by"`
	Title  string  `json:"title"`
	Desc   string  `json:"desc"`
}

// Usage records where a type is consumed.
type Usage struct {
	Service string `json:"service"`
	Method  string `json:"method"`
	Role    string `json:"role"`
}

// Field is one field of a type or of a method's params list.
type Field struct {
	JSONName     string `json:"jsonName"`
	SchemaType   string `json:"schemaType"`
	Required     bool   `json:"required"`
	Position     int    `json:"position"`
	ElemType     string `json:"elemType"`
	TypeName     string `json:"typeName"`
	Nullable     bool   `json:"nullable"`
	OmitEmpty    bool   `json:"omitempty"`
	Title        string `json:"title"`
	Desc         string `json:"desc"`
	Example      string `json:"example"`
	DesignerHint string `json:"designerHint"`
}

// LoadSpec reads and unmarshals an intro.json spec file.
func LoadSpec(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

var goKeywords = map[string]bool{
	"break": true, "default": true, "func": true, "interface": true,
	"select": true, "case": true, "defer": true, "go": true, "map": true,
	"struct": true, "chan": true, "else": true, "goto": true, "package": true,
	"switch": true, "const": true, "fallthrough": true, "if": true,
	"range": true, "type": true, "continue": true, "for": true,
	"import": true, "return": true, "var": true,
}

// GoName converts a spec name into an exported Go identifier.
//
// It splits on any run of non-alphanumeric characters, upper-cases the
// first letter of each segment and concatenates. The segment "id" becomes
// "ID". An empty input yields "".
func GoName(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	start := true
	for _, seg := range strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if seg == "" {
			continue
		}
		if len(seg) == 2 && (seg[0] == 'i' || seg[0] == 'I') && (seg[1] == 'd' || seg[1] == 'D') {
			b.WriteString("ID")
			start = false
			continue
		}
		r := []rune(seg)
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
		start = false
	}
	out := b.String()
	if start {
		return ""
	}
	if goKeywords[out] {
		out += "_"
	}
	return out
}

// FieldInfo is the normalized, Go-ready form of a spec field.
type FieldInfo struct {
	JsonName  string
	GoName    string
	GoType    string
	Required  bool
	OmitEmpty bool
	Doc       string
	Example   string
}

// TypeInfo is the normalized, Go-ready form of a named type.
type TypeInfo struct {
	Name   string
	GoName string
	Fields []FieldInfo
	Doc    string
}

// MethodInfo is the normalized, Go-ready form of an RPC method.
type MethodInfo struct {
	Service      string
	Method       string
	GoName       string
	PostPath     string
	HasParams    bool
	RequestType  string
	ResponseType string
	Title        string
	Params       []FieldInfo
}

// Model is the fully normalized view of a spec.
type Model struct {
	Types   []TypeInfo
	Methods []MethodInfo
}

// FieldGoType maps a spec field to its Go type expression. Slices and maps
// are always non-pointer (nilable); scalars and objects become pointers
// when the field is optional.
func FieldGoType(f *Field) string {
	switch f.SchemaType {
	case "string":
		return scalarType("string", f.Required)
	case "number":
		return scalarType("float64", f.Required)
	case "boolean":
		return scalarType("bool", f.Required)
	case "array":
		switch f.ElemType {
		case "string":
			return "[]string"
		case "number":
			return "[]float64"
		case "boolean":
			return "[]bool"
		case "object":
			return "[]" + GoName(f.TypeName)
		default:
			return "[]any"
		}
	case "object":
		if f.TypeName != "" {
			return scalarType(GoName(f.TypeName), f.Required)
		}
		return "map[string]any"
	default:
		return "any"
	}
}

func scalarType(base string, required bool) string {
	if !required {
		return "*" + base
	}
	return base
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func buildFieldInfo(f *Field) FieldInfo {
	return FieldInfo{
		JsonName:  f.JSONName,
		GoName:    GoName(f.JSONName),
		GoType:    FieldGoType(f),
		Required:  f.Required,
		OmitEmpty: !f.Required || f.OmitEmpty,
		Doc:       oneLine(pickFirst(f.Desc, f.Title)),
		Example:   f.Example,
	}
}

func pickFirst(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// requestTypeName finds the named request type for a service+method via the
// types' used_by entries with role "request".
func requestTypeName(spec *Spec, service, method string) string {
	for name, td := range spec.Types {
		for _, u := range td.UsedBy {
			if u.Role == "request" && u.Service == service && u.Method == method {
				return GoName(name)
			}
		}
	}
	return ""
}

// responseTypeExpr computes the Go return type expression for a method,
// following the response mapping rules.
func responseTypeExpr(spec *Spec, service string, m *Method) (string, error) {
	if m.ResponseTypeName != "" {
		if _, ok := spec.Types[m.ResponseTypeName]; !ok {
			return "", fmt.Errorf("response type %q of %s.%s not in spec types", m.ResponseTypeName, service, m.Method)
		}
		return GoName(m.ResponseTypeName), nil
	}
	if m.ResponseTypeScript == "Record<string, boolean>" {
		return "map[string]bool", nil
	}
	trimmed := strings.TrimSpace(m.ResponseTypeScript)
	if strings.HasSuffix(trimmed, "[]") {
		for name, fields := range m.NestedTypes {
			if len(fields) > 0 {
				return "[]" + GoName(name), nil
			}
		}
		return "", fmt.Errorf("array response %q of %s.%s has no non-empty nested type", trimmed, service, m.Method)
	}
	return "", fmt.Errorf("unsupported response type %q for %s.%s", m.ResponseTypeScript, service, m.Method)
}

// Normalize validates the spec and produces a sorted, Go-ready Model.
func Normalize(spec *Spec) (*Model, error) {
	m := &Model{}

	for name, td := range spec.Types {
		ti := TypeInfo{
			Name:   name,
			GoName: GoName(name),
			Doc:    oneLine(pickFirst(td.Desc, td.Title)),
		}
		for i := range td.Fields {
			ti.Fields = append(ti.Fields, buildFieldInfo(&td.Fields[i]))
		}
		m.Types = append(m.Types, ti)
	}
	sort.Slice(m.Types, func(i, j int) bool {
		return m.Types[i].GoName < m.Types[j].GoName
	})

	svcNames := make([]string, 0, len(spec.Services))
	for name := range spec.Services {
		svcNames = append(svcNames, name)
	}
	sort.Strings(svcNames)

	for _, svcName := range svcNames {
		for _, svc := range spec.Services[svcName] {
			for i := range svc.Methods {
				meth := &svc.Methods[i]
				mi := MethodInfo{
					Service:     svcName,
					Method:      meth.Method,
					GoName:      GoName(svcName) + GoName(meth.Method),
					PostPath:    meth.PostPath,
					HasParams:   meth.HasParams,
					RequestType: requestTypeName(spec, svcName, meth.Method),
					Title:       meth.Title,
				}
				for i := range meth.Params {
					mi.Params = append(mi.Params, buildFieldInfo(&meth.Params[i]))
				}
				if mi.RequestType == "" && meth.HasParams {
					return nil, fmt.Errorf("method %s.%s has params but no request type", svcName, meth.Method)
				}
				rt, err := responseTypeExpr(spec, svcName, meth)
				if err != nil {
					return nil, err
				}
				mi.ResponseType = rt
				m.Methods = append(m.Methods, mi)
			}
		}
	}

	return m, nil
}
