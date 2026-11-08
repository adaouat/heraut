package config

import (
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

var unmarshalerType = reflect.TypeOf((*yaml.Unmarshaler)(nil)).Elem()

// checkBools refuses a non-boolean value for a boolean key. yaml.v3 decodes "yes", "on" and even the
// quoted "yes" into a bool for YAML 1.1 compatibility, while schema.json, like YAML 1.2, accepts only
// true and false. raw is checked against the Config types; a type with its own UnmarshalYAML checks
// itself. A document that doesn't parse is left to the decoder to report.
func checkBools(raw []byte) error {
	var doc yaml.Node
	if yaml.Unmarshal(raw, &doc) != nil || len(doc.Content) == 0 {
		return nil
	}
	var errs []string
	walkBools(doc.Content[0], reflect.TypeOf(Config{}), "", &errs)
	if len(errs) > 0 {
		return fmt.Errorf("config: %s", strings.Join(errs, "; "))
	}
	return nil
}

func walkBools(n *yaml.Node, t reflect.Type, path string, errs *[]string) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if reflect.PointerTo(t).Implements(unmarshalerType) {
		return
	}
	switch t.Kind() {
	case reflect.Bool:
		if n.Kind == yaml.ScalarNode && n.ShortTag() != "!!bool" {
			*errs = append(*errs, fmt.Sprintf("line %d: %s must be true or false, not %q", n.Line, path, n.Value))
		}
	case reflect.Struct:
		if n.Kind != yaml.MappingNode {
			return
		}
		fields := map[string]reflect.Type{}
		for i := range t.NumField() {
			name, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
			if name != "" && name != "-" {
				fields[name] = t.Field(i).Type
			}
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			if ft, ok := fields[n.Content[i].Value]; ok {
				walkBools(n.Content[i+1], ft, joinPath(path, n.Content[i].Value), errs)
			}
		}
	case reflect.Slice:
		if n.Kind == yaml.SequenceNode {
			for i, item := range n.Content {
				walkBools(item, t.Elem(), fmt.Sprintf("%s[%d]", path, i), errs)
			}
		}
	case reflect.Map:
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				walkBools(n.Content[i+1], t.Elem(), joinPath(path, n.Content[i].Value), errs)
			}
		}
	}
}

func joinPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}
