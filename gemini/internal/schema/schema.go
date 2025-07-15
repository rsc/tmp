// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package schema provides reflect-based helpers for generating [genai.Schema]
// descriptions from Go structures and for marshaling and unmarshaling those structures
// into JSON object form of type [any].
package schema

import (
	"fmt"
	"reflect"
	"strings"

	"google.golang.org/genai"
)

// Type returns a [genai.Schema] for the given type.
// The type is usually a struct, or pointer to struct,
// with fields that have `tool:...` tags giving their schema descriptions.
//
// A simple example of an input type is:
//
//	// diffReply is the reply from the AiderSearchReplace and CursorEdit tools.
//	type diffReply struct {
//		Result string    `tool:"A message confirming the replacement was successful."`
//		Diff   string    `tool:"#optional The applied edits in unified diff format."`
//		Files  []newFile `tool:"#optional The complete content of the modified files."`
//	}
//
//	// newFile is a single new file in a diff reply.
//	type newFile struct {
//		Path    string `tool:"The path of the file."`
//		Content string `tool:"The complete new content of the file, with no elided sections."`
//	}
//
// All fields in a struct must be exported, to make them available to reflect,
// but the struct types themselves can be unexported.
//
// The `tool:...` tag gives the schema description for the field.
// If the description begins with #optional, the field is marked optional,
// and that text is omitted from the description.
//
// (The tool tags can end up a bit long, but they are still far shorter than all the
// schema type, marshaling, and unmarshaling code this package lets you
// avoid writing.)
//
// As a special case, if the field is a slice of basic Go types and the
// description contains the marker text " ITEM: ", then the text before the
// marker is used as the description of the field (of schema kind ARRAY),
// while the text after the marker is the description used for each array item.
// For example;
//
//	type cleanBuildArgs struct {
//		WorkspacePaths []string `tool:"The workspace paths of the source code files for which we want to manage dependencies. ITEM: Workspace path of the file to parse (e.g. google3/path/to/file)."`
//	}
//
// This special case is included to ease encoding of existing descriptions,
// but it should not be necessary in new tool descriptions.
// Something like this should work just as well:
//
//	type cleanBuildArgs struct {
//		WorkspacePaths []string `tool:"The workspace paths of the source code files for which we want to manage dependencies (e.g. google3/path/to/file)."`
//	}
//
// The field names used in the schema are converted to conventional
// underscore_names by prefixing every interior upper case letter
// with an underscore and then converting the entire name to lower case.
// For example, "Result" becomes "result", and "FailureReason" becomes "failure_reason".
//
// When preparing a schema, the struct field ordering is used as the PropertyOrdering.
// Fields not marked #optional are listed in the Required list.
//
// The valid Go types for use in schemas are bool, string, integers (except uintptr),
// floats, slices, structs, and pointers. Note that interfaces cannot be used, since
// the schema descriptions do not admit that kind of polymorphism.
// (Maps cannot be used either, for the less fundamental reason of not being implemented.)
//
// Type can be passed any valid Go type, not just structs, but the resulting schema
// will have no description. Using a top-level struct provides a place to write the
// description text.
func Type(typ reflect.Type) (*genai.Schema, error) {
	seen := make(map[reflect.Type]bool)
	return schema(typ, "", seen)
}

// schema returns the [genai.Schema] for the given type and description.
// It returns an error if it encounters any parent type during its recursion,
// since that indicates a cycle that will otherwise never terminate.
func schema(typ reflect.Type, desc string, parent map[reflect.Type]bool) (*genai.Schema, error) {
	// Avoid infinite recursion.
	if parent[typ] {
		return nil, fmt.Errorf("cannot create schema for cyclic type %v", typ)
	}
	parent[typ] = true
	defer func() {
		parent[typ] = false
	}()

	s := &genai.Schema{
		Description: desc,
	}
	switch typ.Kind() {
	default:
		return nil, fmt.Errorf("cannot create schema for %v", typ)

	case reflect.Bool:
		s.Type = genai.TypeBoolean
		return s, nil

	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		s.Type = genai.TypeInteger
		return s, nil

	case reflect.Float32, reflect.Float64:
		s.Type = genai.TypeNumber
		return s, nil

	case reflect.String:
		s.Type = genai.TypeString
		return s, nil

	case reflect.Pointer:
		return schema(typ.Elem(), desc, parent)

	case reflect.Slice:
		items, err := schema(typ.Elem(), "", parent)
		if err != nil {
			return nil, err
		}
		if desc, itemDesc, ok := strings.Cut(desc, " ITEM: "); ok {
			s.Description = strings.TrimSpace(desc)
			items.Description = strings.TrimSpace(itemDesc)
		}
		s.Type = genai.TypeArray
		s.Items = items
		return s, nil

	case reflect.Struct:
		s.Type = genai.TypeObject
		s.Properties = make(map[string]*genai.Schema)
		for i := range typ.NumField() {
			f := typ.Field(i)
			if !f.IsExported() {
				return nil, fmt.Errorf("cannot create schema for type %v with unexported field %v", typ, f.Name)
			}
			name := toSchemaName(f.Name)
			item, err := schema(f.Type, tagDesc(f.Tag), parent)
			if err != nil {
				return nil, err
			}
			s.Properties[name] = item
			s.PropertyOrdering = append(s.PropertyOrdering, name)
			if !isOptional(f.Tag) {
				s.Required = append(s.Required, name)
			}
		}
		return s, nil
	}
}

// Marshal converts a Go value to a JSON value.
// The name argument is used in error messages as the name
// for the top-level value being marshaled.
//
// The Go value must be valid for use with [Type], and then
// Marshal imposes the following restrictions:
//
//   - All numbers being marshaled must be representable exactly as a float64,
//     since that is the underlying type of a JSON number.
//     For example, it is an error to marshal ^uint64(0).
//
//   - All required (non-optional) fields must be present in the corresponding Go value.
//
// When marshaling an optional field, whether the field is considered
// present (and included in the JSON map) depends on the field's type and value.
// If the field has a pointer type, then it is present when non-nil.
// If the field has a non-pointer type, then it is present when it is not the zero value for that type.
// When it is important to distinguish a "present zero" value from an "omitted" value,
// use a pointer.
func Marshal(v any, name string) (any, error) {
	return marshal(reflect.ValueOf(v), name)
}

func marshal(v reflect.Value, path string) (any, error) {
	switch v.Kind() {
	default:
		return nil, fmt.Errorf("cannot encode type %v for %s", v.Type(), path)

	case reflect.Bool:
		return v.Bool(), nil

	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		i := v.Int()
		f := float64(i)
		if int64(f) != i {
			return nil, fmt.Errorf("cannot represent %v as number for %s", i, path)
		}
		return f, nil

	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		u := v.Uint()
		f := float64(u)
		if uint64(f) != u {
			return nil, fmt.Errorf("cannot represent %v as number for %s", u, path)
		}
		return f, nil

	case reflect.Float32, reflect.Float64:
		return v.Float(), nil

	case reflect.String:
		return v.String(), nil

	case reflect.Pointer:
		if v.IsNil() {
			return nil, fmt.Errorf("cannot encode nil %v for %s", v.Type(), path)
		}
		return marshal(v.Elem(), path)

	case reflect.Slice:
		var list []any
		for i := range v.Len() {
			elem, err := marshal(v.Index(i), fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			list = append(list, elem)
		}
		return list, nil

	case reflect.Struct:
		t := v.Type()
		obj := make(map[string]any)
		for i := range t.NumField() {
			f := t.Field(i)
			name := toSchemaName(f.Name)
			vf := v.Field(i)
			if isOptional(f.Tag) && vf.IsZero() {
				continue
			}
			if vf.Kind() == reflect.Pointer && vf.IsNil() {
				// marshal would error out on the nil pointer too, but this message is clearer.
				return nil, fmt.Errorf("missing required field %s.%s", path, name)
			}
			elem, err := marshal(v.Field(i), path+"."+name)
			if err != nil {
				return nil, err
			}
			obj[name] = elem
		}
		return obj, nil
	}
}

// Unmarshal decodes a JSON value into v, which must be a pointer to some type.
// The pointed-to value will be zeroed and then filled in.
// The name argument is used in error messages as the name
// for the top-level value being unmarshaled.
//
// The Go value type must be valid for use with [Type], and then Unmarshal
// imposes the following restrictions:
//
//   - All incoming values must have a kind appropriate to the Go type.
//     For example, it is an error to unmarshal "hello world" or "123" or true into an integer.
//
//   - All incoming values must fit in the corresponding Go type.
//     For example, it is an error to unmarshal the number 1.5 into an int
//     or to unmarshal the number 1000 into a uint8.
//
//   - All required (non-optional) fields must be present in the corresponding JSON map.
func Unmarshal(js, v any, name string) error {
	rv, err := unmarshalValueOf(v)
	if err != nil {
		return err
	}
	return unmarshal(js, rv, name)
}

// unmarshalValueOf returns the reflect.Value to use for unmarshaling.
// It is roughly reflect.ValueOf(v).Elem() but checks for non-pointers
// and zeros the pointed-at memory.
func unmarshalValueOf(v any) (reflect.Value, error) {
	// Note: Not worried about using name in these errors because these
	// indicate bugs in the tool code, passing an entirely wrong argument
	// to Unmarshal. These should not be observed by the model.
	if v == nil {
		return reflect.Value{}, fmt.Errorf("cannot unmarshal into nil interface value")
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer {
		return reflect.Value{}, fmt.Errorf("cannot unmarshal into non-pointer of type %v", rv.Type())
	}
	if rv.IsNil() {
		return reflect.Value{}, fmt.Errorf("cannot unmarshal into nil pointer of type %v", rv.Type())
	}
	elem := rv.Elem()
	elem.Set(reflect.New(elem.Type()).Elem()) // zero elem
	return elem, nil
}

func unmarshal(js any, v reflect.Value, path string) error {
	t := v.Type()
	switch v.Kind() {
	case reflect.Bool:
		b, ok := js.(bool)
		if !ok {
			break
		}
		v.SetBool(b)
		return nil

	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		f, ok := js.(float64)
		if !ok {
			break
		}
		i := int64(f)
		if float64(i) != f {
			return fmt.Errorf("cannot use %v as integer for %s", f, path)
		}
		if t.OverflowInt(i) {
			return fmt.Errorf("cannot use %v as %v for %s", i, t, path)
		}
		v.SetInt(i)
		return nil

	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		f, ok := js.(float64)
		if !ok {
			break
		}
		u := uint64(f)
		if float64(u) != f {
			return fmt.Errorf("cannot use %v as unsigned integer for %s", f, path)
		}
		if t.OverflowUint(u) {
			return fmt.Errorf("cannot use %v as %v for %s", u, t, path)
		}
		v.SetUint(u)
		return nil

	case reflect.Float32, reflect.Float64:
		f, ok := js.(float64)
		if !ok {
			break
		}
		if t.OverflowFloat(f) {
			return fmt.Errorf("cannot use %v as %v for %s", f, t, path)
		}
		v.SetFloat(f)
		return nil

	case reflect.String:
		s, ok := js.(string)
		if !ok {
			break
		}
		v.SetString(s)
		return nil

	case reflect.Pointer:
		v.Set(reflect.New(t.Elem()))
		return unmarshal(js, v.Elem(), path)

	case reflect.Slice:
		values, ok := js.([]any)
		if !ok {
			return fmt.Errorf("cannot use %s value as list for %s", jsKind(js), path)
		}
		v.Set(reflect.MakeSlice(v.Type(), len(values), len(values)))
		for i, jv := range values {
			if err := unmarshal(jv, v.Index(i), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
		return nil

	case reflect.Struct:
		fields, ok := js.(map[string]any)
		if !ok {
			return fmt.Errorf("cannot use %s value as struct for %s", jsKind(js), path)
		}
		t := v.Type()
		for i := range t.NumField() {
			f := t.Field(i)
			name := toSchemaName(f.Name)
			sv, ok := fields[name]
			if !ok {
				if isOptional(f.Tag) {
					continue
				}
				return fmt.Errorf("unmarshal missing field %s.%s", path, name)
			}
			if err := unmarshal(sv, v.Field(i), path+"."+name); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("cannot use %s value as %v for %s", jsKind(js), t, path)
}

// jsKind returns an LLM-readable description of the kind of a value.
func jsKind(kind any) string {
	switch kind.(type) {
	default:
		return reflect.TypeOf(kind).String()
	case nil:
		return "missing"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case map[string]any:
		return "object" // schema calls it object, not struct
	case []any:
		return "list"
	}
}

// isOptional reports whether the field's tag marks it optional.
func isOptional(tag reflect.StructTag) bool {
	return strings.HasPrefix(tag.Get("tool"), "#optional ")
}

// tagDesc returns the field's tag description.
func tagDesc(tag reflect.StructTag) string {
	return strings.TrimPrefix(tag.Get("tool"), "#optional ")
}

// toSchemaName converts the Go field name to a schema name.
// It inserts an underscore between every non-capital letter followed
// by a capital letter and then lower-cases the entire name.
func toSchemaName(name string) string {
	out := make([]byte, 0, len(name)*2)
	last := byte('A')
	for i := range len(name) {
		c := name[i]
		lower := c
		if 'A' <= c && c <= 'Z' {
			if !('A' <= last && last <= 'Z') {
				out = append(out, '_')
			}
			lower += 'a' - 'A'
		}
		out = append(out, lower)
		last = c
	}
	return string(out)
}
