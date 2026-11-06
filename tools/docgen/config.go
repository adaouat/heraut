package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	beginMarker = "<!-- BEGIN GENERATED from schema.json by `mise run docs:gen`. Edit schema.json, not this part. -->"
	endMarker   = "<!-- END GENERATED -->"
)

// replaceGenerated swaps the part of doc between the markers for generated, leaving the
// hand-written text around it alone.
func replaceGenerated(doc, generated string) (string, error) {
	i := strings.Index(doc, beginMarker)
	j := strings.Index(doc, endMarker)
	if i < 0 || j < i {
		return "", errors.New("the generated-section markers are missing")
	}
	return doc[:i+len(beginMarker)] + "\n" + generated + doc[j:], nil
}

type node map[string]json.RawMessage

func (n node) str(key string) string {
	var s string
	_ = json.Unmarshal(n[key], &s)
	return s
}

func (n node) child(key string) node {
	var c node
	_ = json.Unmarshal(n[key], &c)
	return c
}

// keysInOrder returns the keys of a JSON object in file order: the reference follows the schema's
// own order, which encoding/json maps lose.
func keysInOrder(raw json.RawMessage) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		keys = append(keys, tok.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil, err
		}
	}
	return keys, nil
}

type refGen struct {
	defs map[string]node
	seen map[string]string // definition name → the path it was first expanded at
	b    strings.Builder
}

// resolve follows a $ref, returning the definition and its name ("" when n is not a reference).
func (g *refGen) resolve(n node) (node, string) {
	if ref := n.str("$ref"); ref != "" {
		name := strings.TrimPrefix(ref, "#/definitions/")
		return g.defs[name], name
	}
	return n, ""
}

func (g *refGen) description(n node) string {
	if d := n.str("description"); d != "" {
		return d
	}
	def, _ := g.resolve(n)
	return def.str("description")
}

func typeCell(n node, resolved node) string {
	var enum []any
	if json.Unmarshal(n["enum"], &enum) == nil && len(enum) > 0 {
		vals := make([]string, len(enum))
		for i, v := range enum {
			vals[i] = fmt.Sprintf("`%v`", v)
		}
		return strings.Join(vals, ` \| `)
	}
	switch t := resolved.str("type"); t {
	case "array":
		items := resolved.child("items")
		if it := items.str("type"); it != "" {
			return "list of " + it
		}
		return "list of objects"
	case "":
		return "object"
	default:
		return t
	}
}

func defaultCell(n node) string {
	raw, ok := n["default"]
	if !ok {
		return ""
	}
	var v any
	_ = json.Unmarshal(raw, &v)
	return fmt.Sprintf("`%v`", v)
}

func cell(s string) string {
	return strings.NewReplacer("|", `\|`, "<", `\<`, ">", `\>`, "\n", " ").Replace(s)
}

// children writes a row per key under n, depth first. A definition already expanded elsewhere is
// referred to instead of repeated, so environments.<name>.changelog points at changelog.
func (g *refGen) children(path string, n node) error {
	resolved, _ := g.resolve(n)
	if items := resolved.child("items"); items != nil {
		return g.expand(path+"[]", items)
	}
	if extra := resolved.child("additionalProperties"); extra != nil {
		return g.expand(path+".<name>", extra)
	}
	return g.expand(path, n)
}

func (g *refGen) expand(path string, n node) error {
	resolved, defName := g.resolve(n)
	if defName != "" {
		if _, done := g.seen[defName]; !done {
			g.seen[defName] = path
		}
	}
	props, ok := resolved["properties"]
	if !ok {
		return nil
	}
	keys, err := keysInOrder(props)
	if err != nil {
		return err
	}
	all := resolved.child("properties")
	for _, k := range keys {
		child := all.child(k)
		childPath := k
		if path != "" {
			childPath = path + "." + k
		}
		childResolved, childDef := g.resolve(child)
		desc := g.description(child)
		first, expanded := g.seen[childDef]
		if childDef != "" && expanded {
			desc += fmt.Sprintf(" Same keys as `%s`.", first)
		}
		fmt.Fprintf(&g.b, "| `%s` | %s | %s | %s |\n", childPath, typeCell(child, childResolved), defaultCell(child), cell(desc))
		if childDef != "" && expanded {
			continue
		}
		if err := g.children(childPath, child); err != nil {
			return err
		}
	}
	return nil
}

// configReference renders schema.json as one section per top-level key, each a table of its keys:
// path, type, default and description.
func configReference(schema []byte) (string, error) {
	var root node
	if err := json.Unmarshal(schema, &root); err != nil {
		return "", fmt.Errorf("parsing the schema: %w", err)
	}
	g := &refGen{defs: map[string]node{}, seen: map[string]string{}}
	var defs map[string]node
	if err := json.Unmarshal(root["definitions"], &defs); err != nil && root["definitions"] != nil {
		return "", fmt.Errorf("parsing the definitions: %w", err)
	}
	g.defs = defs

	keys, err := keysInOrder(root["properties"])
	if err != nil {
		return "", err
	}
	props := root.child("properties")
	// A definition that has its own top-level key is expanded there; anywhere else it is referred to,
	// even when an earlier key reaches it first (changelog.rendering points at rendering).
	for _, k := range keys {
		if _, def := g.resolve(props.child(k)); def != "" {
			if _, done := g.seen[def]; !done {
				g.seen[def] = k
			}
		}
	}
	const header = "| Key | Type | Default | Description |\n|---|---|---|---|\n"
	for _, k := range keys {
		prop := props.child(k)
		resolved, _ := g.resolve(prop)
		_, hasProps := resolved["properties"]
		nested := hasProps || resolved.child("items") != nil || resolved.child("additionalProperties") != nil
		fmt.Fprintf(&g.b, "\n## `%s`\n\n", k)
		if !nested {
			fmt.Fprintf(&g.b, "%s| `%s` | %s | %s | %s |\n", header, k, typeCell(prop, resolved), defaultCell(prop), cell(g.description(prop)))
			continue
		}
		fmt.Fprintf(&g.b, "%s\n\n%s", cell(g.description(prop)), header)
		if err := g.children(k, prop); err != nil {
			return "", err
		}
	}
	return g.b.String(), nil
}
