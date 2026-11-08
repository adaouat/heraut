package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	forgeconfig "github.com/adaouat/forge/config"
	"gopkg.in/yaml.v3"
)

// decodeConfig strictly decodes raw into cfg, as forge's config.Decode does, but keeps yaml's type
// error so each unknown key can be reported by its place in the file and the key probably meant
// (describeUnknownKey), instead of a Go type name.
func decodeConfig(raw []byte, cfg *Config) error {
	if err := checkBools(raw); err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	err := dec.Decode(cfg)
	if err == nil {
		return nil
	}
	if errors.Is(err, io.EOF) {
		return forgeconfig.ErrEmptyConfig
	}
	var typeErr *yaml.TypeError
	if errors.As(err, &typeErr) {
		msgs := make([]string, len(typeErr.Errors))
		for i, e := range typeErr.Errors {
			msgs[i] = describeUnknownKey(raw, e)
		}
		return fmt.Errorf("config: %s", strings.Join(msgs, "; "))
	}
	return fmt.Errorf("config: %s", describeUnknownKey(raw, err.Error()))
}

var unknownFieldRe = regexp.MustCompile(`^line (\d+): field "?([^"\s]+)"? not found in type config\.(\w+)$`)

// describeUnknownKey turns yaml's "line 4: field tag_prefx not found in type config.Versioning" into
// `line 4: unknown key "tag_prefx" in versioning — did you mean "tag_prefix"?`. Any other message is
// returned as it is.
func describeUnknownKey(raw []byte, msg string) string {
	m := unknownFieldRe.FindStringSubmatch(msg)
	if m == nil {
		return msg
	}
	line, _ := strconv.Atoi(m[1])
	key, typeName := m[2], m[3]
	where := "at the top level"
	if path := keyPath(raw, line, key); path != "" {
		where = "in " + path
	}
	out := fmt.Sprintf("line %d: unknown key %q %s", line, key, where)
	if s := closestKey(key, yamlKeysOf(typeName)); s != "" {
		out += fmt.Sprintf(" — did you mean %q?", s)
	}
	return out
}

// keyPath is the dotted path of the mapping holding key at line, e.g. "forges[0]"; "" at the top level.
func keyPath(raw []byte, line int, key string) string {
	var doc yaml.Node
	if yaml.Unmarshal(raw, &doc) != nil || len(doc.Content) == 0 {
		return ""
	}
	var find func(n *yaml.Node, path string) (string, bool)
	find = func(n *yaml.Node, path string) (string, bool) {
		switch n.Kind {
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				k, v := n.Content[i], n.Content[i+1]
				if k.Line == line && k.Value == key {
					return path, true
				}
				child := k.Value
				if path != "" {
					child = path + "." + k.Value
				}
				if p, ok := find(v, child); ok {
					return p, true
				}
			}
		case yaml.SequenceNode:
			for i, item := range n.Content {
				if p, ok := find(item, fmt.Sprintf("%s[%d]", path, i)); ok {
					return p, true
				}
			}
		}
		return "", false
	}
	p, _ := find(doc.Content[0], "")
	return p
}

// yamlKeysOf lists the YAML keys of the config struct named typeName.
func yamlKeysOf(typeName string) []string {
	seen := map[reflect.Type]bool{}
	var keys []string
	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Map {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct || seen[t] {
			return
		}
		seen[t] = true
		for i := range t.NumField() {
			f := t.Field(i)
			name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
			if t.Name() == typeName && name != "" && name != "-" {
				keys = append(keys, name)
			}
			walk(f.Type)
		}
	}
	walk(reflect.TypeOf(Config{}))
	return keys
}

// closestKey is the known key within two edits of key, or "" when none is that close.
func closestKey(key string, known []string) string {
	best, bestDist := "", 3
	for _, k := range known {
		if d := editDistance(key, k); d < bestDist {
			best, bestDist = k, d
		}
	}
	return best
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
