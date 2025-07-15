// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schema

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/genai"
)

func TestToSchemaName(t *testing.T) {
	for _, tt := range toSchemaNameTests {
		out := toSchemaName(tt.in)
		if out != tt.out {
			t.Errorf("toSchemaName(%q) = %q, want %q", tt.in, out, tt.out)
		}
	}
}

var toSchemaNameTests = []struct {
	in  string
	out string
}{
	{"Result", "result"},
	{"FailureReason", "failure_reason"},
	{"URL", "url"},
	{"URLs", "urls"},
	{"URLsForMe", "urls_for_me"},
	{"MyID", "my_id"},
}

func TestType(t *testing.T) {
	for _, tt := range typeTests {
		out, err := Type(tt.typ)
		if err != nil {
			t.Errorf("Type(%v): %v", tt.typ, err)
			continue
		}
		if diff := cmp.Diff(tt.schema, out); diff != "" {
			t.Errorf("Type(%v) schema diff:\n%s", tt.typ, diff)
		}
	}
}

var typeTests = []struct {
	typ    reflect.Type
	schema *genai.Schema
}{
	// All basic types, each wrapped in a struct.
	{testStruct[bool](), testSchema(genai.TypeBoolean)},
	{testStruct[namedBool](), testSchema(genai.TypeBoolean)},
	{testStruct[int](), testSchema(genai.TypeInteger)},
	{testStruct[int8](), testSchema(genai.TypeInteger)},
	{testStruct[int16](), testSchema(genai.TypeInteger)},
	{testStruct[int32](), testSchema(genai.TypeInteger)},
	{testStruct[namedInt32](), testSchema(genai.TypeInteger)},
	{testStruct[int64](), testSchema(genai.TypeInteger)},
	{testStruct[uint](), testSchema(genai.TypeInteger)},
	{testStruct[uint8](), testSchema(genai.TypeInteger)},
	{testStruct[uint16](), testSchema(genai.TypeInteger)},
	{testStruct[uint32](), testSchema(genai.TypeInteger)},
	{testStruct[namedUint32](), testSchema(genai.TypeInteger)},
	{testStruct[uint64](), testSchema(genai.TypeInteger)},
	{testStruct[float32](), testSchema(genai.TypeNumber)},
	{testStruct[float64](), testSchema(genai.TypeNumber)},
	{testStruct[namedFloat64](), testSchema(genai.TypeNumber)},
	{testStruct[string](), testSchema(genai.TypeString)},
	{testStruct[namedString](), testSchema(genai.TypeString)},

	// Composite types.
	{reflect.TypeFor[sliceStruct](), sliceSchema},
	{reflect.TypeFor[optStruct](), optSchema},

	// Actual converted schemas.
	{reflect.TypeFor[*cleanBuildArgs](), cleanBuildArgsSchema},
	{reflect.TypeFor[*cleanBuildReply](), cleanBuildReplySchema},
	{reflect.TypeFor[codeSearchArgs](), codeSearchArgsSchema},
	{reflect.TypeFor[codeSearchReply](), codeSearchReplySchema},
}

func TestTypeErrors(t *testing.T) {
	for _, tt := range typeErrorTests {
		_, err := Type(tt.typ)
		if err == nil {
			t.Errorf("Type %v succeeded unexpectedly", tt.typ)
			continue
		}
		if !strings.Contains(err.Error(), tt.err) {
			t.Errorf("Type %v: error %q, want %q", tt.typ, err, tt.err)
		}
	}
}

var typeErrorTests = []struct {
	typ reflect.Type
	err string
}{
	{reflect.TypeFor[linkedList](), "cannot create schema for cyclic type schema.linkedList"},
	{reflect.TypeFor[*outerList](), "cannot create schema for cyclic type *schema.linkedList"},
	{reflect.TypeFor[*any](), "cannot create schema for interface {}"},
	{reflect.TypeFor[*unexportedFields](), "cannot create schema for type schema.unexportedFields with unexported field foo"},
	{reflect.TypeFor[[]*unexportedFields](), "cannot create schema for type schema.unexportedFields with unexported field foo"},
}

func TestMarshal(t *testing.T) {
	for _, tt := range marshalTests {
		out, err := Marshal(tt.in, "NAME")
		if err != nil {
			t.Errorf("Marshal %T: %v", tt.in, err)
			continue
		}
		if diff := cmp.Diff(out, tt.out, cmpopts.EquateNaNs()); diff != "" {
			t.Errorf("Marshal %T diff -have +want:\n%s", tt.in, diff)
			continue
		}
		ptr := reflect.New(reflect.TypeOf(tt.in)).Interface()
		if err := Unmarshal(out, ptr, "NAME"); err != nil {
			t.Errorf("Unmarshal %T: %v", tt.in, err)
			continue
		}

		// Special case: PtrOptSlice ptr([]string(nil)) can unmarshal to ptr([]string{});
		// change it back for cmp.Diff.
		// (cmp.Diff does not let us express that nil becoming non-nil is okay,
		// but non-nil becoming nil in OptSlice is NOT okay.)
		if ptr, ok := ptr.(**optStruct); ok {
			v := *ptr
			if v.PtrOptSlice != nil && len(*v.PtrOptSlice) == 0 {
				*v.PtrOptSlice = nil
			}
		}
		if diff := cmp.Diff(reflect.ValueOf(ptr).Elem().Interface(), tt.in, cmpopts.EquateNaNs()); diff != "" {
			t.Errorf("Marshal round trip %T diff -have +want:\n%s", tt.in, diff)
		}
	}
}

func js(x any) any {
	switch x := x.(type) {
	default:
		panic(fmt.Sprintf("js %T", x))
	case bool, float64, string:
		return x
	case int:
		return float64(x)
	}
}

var marshalTests = []struct {
	in  any
	out any
}{
	{false, js(false)},
	{true, js(true)},
	{int(123), js(123)},
	{int8(123), js(123)},
	{int16(123), js(123)},
	{int32(123), js(123)},
	{int64(123), js(123)},
	{uint(123), js(123)},
	{uint8(123), js(123)},
	{uint16(123), js(123)},
	{uint32(123), js(123)},
	{uint64(123), js(123)},
	{float32(123), js(123)},
	{float64(123), js(123)},
	{float32(0.1), js(f32(0.1))},
	{float64(0.1), js(0.1)},
	{"abc", js("abc")},
	{ptr(true), js(true)},
	{ptr(123), js(123)},
	{ptr("abc"), js("abc")},
	{[]string{"abc", "def"}, list(js("abc"), js("def"))},
	{new(optStruct), object(map[string]any{})},
	{optStructFull, optStructFullValue},
	{optStructTricky, optStructTrickyValue},
	{math.NaN(), js(math.NaN())},
	{math.Inf(+1), js(math.Inf(+1))},
	{math.Inf(-1), js(math.Inf(-1))},
}

func TestMarshalError(t *testing.T) {
	for _, tt := range marshalErrorTests {
		_, err := Marshal(tt.in, "NAME")
		if err == nil {
			t.Errorf("Marshal %T %v succeeded unexpectedly", tt.in, tt.in)
			continue
		}
		if err.Error() != tt.err {
			t.Errorf("Marshal %T %v: error %q, want %q", tt.in, tt.in, err, tt.err)
		}
	}
}

var marshalErrorTests = []struct {
	in  any
	err string
}{
	{int64(^uint64(0) >> 2), "cannot represent 4611686018427387903 as number for NAME"},
	{^uint64(2), "cannot represent 18446744073709551613 as number for NAME"},
	{(*int8)(nil), "cannot encode nil *int8 for NAME"},
	{[]uint64{^uint64(2)}, "cannot represent 18446744073709551613 as number for NAME[0]"},
	{oneField[*string]{}, "missing required field NAME.field"},
	{optStruct{OptUint64: ^uint64(2)}, "cannot represent 18446744073709551613 as number for NAME.opt_uint64"},
	{new(any), "cannot encode type interface {} for NAME"},
}

func TestUnmarshal(t *testing.T) {
	// Note: Most tests of successful Unmarshal are handled by TestMarshal's round trip.
	// This test is only handling special cases that Marshal does not generate.
	for _, tt := range unmarshalTests {
		ptr := reflect.New(reflect.TypeOf(tt.out)).Interface()
		err := Unmarshal(tt.in, ptr, "NAME")
		if err != nil {
			t.Errorf("Unmarshal %T: %v", tt.in, err)
			continue
		}
		elem := reflect.ValueOf(ptr).Elem().Interface()
		if diff := cmp.Diff(elem, tt.out, cmpopts.EquateNaNs()); diff != "" {
			t.Errorf("Unmarshal %T diff -have +want:\n%s", tt.in, diff)
		}
		// Note: No round trip here, since these test cases are
		// special values that cannot be generated exactly by Marshal.
	}
}

// Note: Values that can be generated by Marshal should be listed in marshalTests instead.
// These test are only special cases that cannot be generated by Marshal.
var unmarshalTests = []struct {
	in  any
	out any
}{
	{js(0.1), float64(0.1)},      // exact, round trips
	{js(0.1), float32(f32(0.1))}, // inexact, okay but doesn't round trip
}

func TestUnmarshalError(t *testing.T) {
	// Note: Most tests of successful Unmarshal are handled by TestMarshal's round trip.
	// This test is only handling special cases that Marshal does not generate.
	for _, tt := range unmarshalErrorTests {
		err := Unmarshal(tt.in, tt.ptr, "NAME")
		if err == nil {
			t.Errorf("Unmarshal %T %v succeeded unexpectedly", tt.ptr, tt.in)
			continue
		}
		if err.Error() != tt.err {
			t.Errorf("Unmarshal %T %v: error %q, want %q", tt.ptr, tt.in, err, tt.err)
		}
	}
}

var unmarshalErrorTests = []struct {
	in  any
	ptr any
	err string
}{
	// call site errors, meant for user
	{js(false), nil, "cannot unmarshal into nil interface value"},
	{js(false), 1, "cannot unmarshal into non-pointer of type int"},
	{js(false), (*int)(nil), "cannot unmarshal into nil pointer of type *int"},

	// data errors, meant for LLM
	{js(false), new(int), "cannot use boolean value as int for NAME"},
	{js(false), new(uint), "cannot use boolean value as uint for NAME"},
	{js(false), new(float64), "cannot use boolean value as float64 for NAME"},
	{js(false), new(string), "cannot use boolean value as string for NAME"},
	{js(1.5), new(int), "cannot use 1.5 as integer for NAME"},
	{js(1000), new(int8), "cannot use 1000 as int8 for NAME"},
	{js(1.5), new(uint), "cannot use 1.5 as unsigned integer for NAME"},
	{js(1000), new(uint8), "cannot use 1000 as uint8 for NAME"},
	{js(-1), new(uint8), "cannot use -1 as unsigned integer for NAME"},
	{js(1e300), new(float32), "cannot use 1e+300 as float32 for NAME"},
	{js(1), new(bool), "cannot use number value as bool for NAME"},
	{js(1), new([]int), "cannot use number value as list for NAME"},
	{list(js(1)), new([]bool), "cannot use number value as bool for NAME[0]"},
	{js(1), new(optStruct), "cannot use number value as struct for NAME"},
	{object(nil), new(oneField[int]), "unmarshal missing field NAME.field"},
	{object(map[string]any{"Field": js(1)}), new(oneField[int]), "unmarshal missing field NAME.field"},
	{object(map[string]any{"field": js(1)}), new(oneField[bool]), "cannot use number value as bool for NAME.field"},
}

func triv(typ genai.Type, desc string) *genai.Schema {
	return &genai.Schema{Type: typ, Description: desc}
}

func ptr[T any](x T) *T { return &x }

func list(values ...any) any {
	return values
}

func object(fields map[string]any) any {
	return fields
}

type oneField[T any] struct {
	Field T `tool:"the description"`
}

type (
	namedBool    bool
	namedInt32   int32
	namedUint32  uint32
	namedString  string
	namedFloat64 float64
)

type outerList struct {
	List *linkedList
}

type linkedList struct {
	Next *linkedList
}

type unexportedFields struct {
	foo string
}

func testStruct[T any]() reflect.Type {
	return reflect.TypeFor[oneField[T]]()
}

func testSchema(typ genai.Type) *genai.Schema {
	return &genai.Schema{
		Type:             genai.TypeObject,
		Properties:       map[string]*genai.Schema{"field": triv(typ, "the description")},
		PropertyOrdering: []string{"field"},
		Required:         []string{"field"},
	}
}

type sliceStruct struct {
	SliceOfStrings []string `tool:"strings to print"`
	Items          []string `tool:"strings to print ITEM: the string to print"` // deprecated form
}

var sliceSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"slice_of_strings": &genai.Schema{
			Type:        genai.TypeArray,
			Description: "strings to print",
			Items:       triv(genai.TypeString, ""),
		},
		"items": &genai.Schema{
			Type:        genai.TypeArray,
			Description: "strings to print",
			Items:       triv(genai.TypeString, "the string to print"),
		},
	},
	PropertyOrdering: []string{"slice_of_strings", "items"},
	Required:         []string{"slice_of_strings", "items"},
}

type optStruct struct {
	OptBool       bool           `tool:"#optional bool value"`
	OptInt        int            `tool:"#optional int value"`
	OptInt8       int8           `tool:"#optional int8 value"`
	OptInt16      int16          `tool:"#optional int16 value"`
	OptInt32      int32          `tool:"#optional int32 value"`
	OptInt64      int64          `tool:"#optional int64 value"`
	OptUint       uint           `tool:"#optional uint value"`
	OptUint8      uint8          `tool:"#optional uint8 value"`
	OptUint16     uint16         `tool:"#optional uint16 value"`
	OptUint32     uint32         `tool:"#optional uint32 value"`
	OptUint64     uint64         `tool:"#optional uint64 value"`
	OptFloat32    float32        `tool:"#optional float32 value"`
	OptFloat64    float64        `tool:"#optional float64 value"`
	OptString     string         `tool:"#optional string value"`
	OptSlice      []string       `tool:"#optional slice value"`
	OptStruct     oneField[int]  `tool:"#optional struct value"`
	PtrOptBool    *bool          `tool:"#optional bool ptr value"`
	PtrOptInt     *int           `tool:"#optional int ptr value"`
	PtrOptInt8    *int8          `tool:"#optional int8 ptr value"`
	PtrOptInt16   *int16         `tool:"#optional int16 ptr value"`
	PtrOptInt32   *int32         `tool:"#optional int32 ptr value"`
	PtrOptInt64   *int64         `tool:"#optional int64 ptr value"`
	PtrOptUint    *uint          `tool:"#optional uint ptr value"`
	PtrOptUint8   *uint8         `tool:"#optional uint8 ptr value"`
	PtrOptUint16  *uint16        `tool:"#optional uint16 ptr value"`
	PtrOptUint32  *uint32        `tool:"#optional uint32 ptr value"`
	PtrOptUint64  *uint64        `tool:"#optional uint64 ptr value"`
	PtrOptFloat32 *float32       `tool:"#optional float32 ptr value"`
	PtrOptFloat64 *float64       `tool:"#optional float64 ptr value"`
	PtrOptString  *string        `tool:"#optional string ptr value"`
	PtrOptSlice   *[]string      `tool:"#optional slice ptr value"`
	PtrOptStruct  *oneField[int] `tool:"#optional struct ptr value"`
}

var optSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"opt_bool":    triv(genai.TypeBoolean, "bool value"),
		"opt_int":     triv(genai.TypeInteger, "int value"),
		"opt_int8":    triv(genai.TypeInteger, "int8 value"),
		"opt_int16":   triv(genai.TypeInteger, "int16 value"),
		"opt_int32":   triv(genai.TypeInteger, "int32 value"),
		"opt_int64":   triv(genai.TypeInteger, "int64 value"),
		"opt_uint":    triv(genai.TypeInteger, "uint value"),
		"opt_uint8":   triv(genai.TypeInteger, "uint8 value"),
		"opt_uint16":  triv(genai.TypeInteger, "uint16 value"),
		"opt_uint32":  triv(genai.TypeInteger, "uint32 value"),
		"opt_uint64":  triv(genai.TypeInteger, "uint64 value"),
		"opt_float32": triv(genai.TypeNumber, "float32 value"),
		"opt_float64": triv(genai.TypeNumber, "float64 value"),
		"opt_string":  triv(genai.TypeString, "string value"),
		"opt_slice": &genai.Schema{
			Type:        genai.TypeArray,
			Description: "slice value",
			Items:       &genai.Schema{Type: genai.TypeString},
		},
		"opt_struct": &genai.Schema{
			Type:             genai.TypeObject,
			Description:      "struct value",
			Properties:       map[string]*genai.Schema{"field": triv(genai.TypeInteger, "the description")},
			PropertyOrdering: []string{"field"},
			Required:         []string{"field"},
		},
		"ptr_opt_bool":    triv(genai.TypeBoolean, "bool ptr value"),
		"ptr_opt_int":     triv(genai.TypeInteger, "int ptr value"),
		"ptr_opt_int8":    triv(genai.TypeInteger, "int8 ptr value"),
		"ptr_opt_int16":   triv(genai.TypeInteger, "int16 ptr value"),
		"ptr_opt_int32":   triv(genai.TypeInteger, "int32 ptr value"),
		"ptr_opt_int64":   triv(genai.TypeInteger, "int64 ptr value"),
		"ptr_opt_uint":    triv(genai.TypeInteger, "uint ptr value"),
		"ptr_opt_uint8":   triv(genai.TypeInteger, "uint8 ptr value"),
		"ptr_opt_uint16":  triv(genai.TypeInteger, "uint16 ptr value"),
		"ptr_opt_uint32":  triv(genai.TypeInteger, "uint32 ptr value"),
		"ptr_opt_uint64":  triv(genai.TypeInteger, "uint64 ptr value"),
		"ptr_opt_float32": triv(genai.TypeNumber, "float32 ptr value"),
		"ptr_opt_float64": triv(genai.TypeNumber, "float64 ptr value"),
		"ptr_opt_string":  triv(genai.TypeString, "string ptr value"),
		"ptr_opt_slice": &genai.Schema{
			Type:        genai.TypeArray,
			Description: "slice ptr value",
			Items:       &genai.Schema{Type: genai.TypeString},
		},
		"ptr_opt_struct": &genai.Schema{
			Type:             genai.TypeObject,
			Description:      "struct ptr value",
			Properties:       map[string]*genai.Schema{"field": triv(genai.TypeInteger, "the description")},
			PropertyOrdering: []string{"field"},
			Required:         []string{"field"},
		},
	},
	PropertyOrdering: []string{
		"opt_bool",
		"opt_int",
		"opt_int8",
		"opt_int16",
		"opt_int32",
		"opt_int64",
		"opt_uint",
		"opt_uint8",
		"opt_uint16",
		"opt_uint32",
		"opt_uint64",
		"opt_float32",
		"opt_float64",
		"opt_string",
		"opt_slice",
		"opt_struct",
		"ptr_opt_bool",
		"ptr_opt_int",
		"ptr_opt_int8",
		"ptr_opt_int16",
		"ptr_opt_int32",
		"ptr_opt_int64",
		"ptr_opt_uint",
		"ptr_opt_uint8",
		"ptr_opt_uint16",
		"ptr_opt_uint32",
		"ptr_opt_uint64",
		"ptr_opt_float32",
		"ptr_opt_float64",
		"ptr_opt_string",
		"ptr_opt_slice",
		"ptr_opt_struct",
	},
}

var optStructFull = &optStruct{
	OptBool:       true,
	OptInt:        1,
	OptInt8:       2,
	OptInt16:      -3,
	OptInt32:      4,
	OptInt64:      -5,
	OptUint:       6,
	OptUint8:      7,
	OptUint16:     8,
	OptUint32:     9,
	OptUint64:     10,
	OptFloat32:    11.1,
	OptFloat64:    12.1,
	OptString:     "abc",
	OptSlice:      []string{"def", "ghi"},
	OptStruct:     oneField[int]{13},
	PtrOptBool:    ptr(false),
	PtrOptInt:     ptr(int(14)),
	PtrOptInt8:    ptr(int8(15)),
	PtrOptInt16:   ptr(int16(16)),
	PtrOptInt32:   ptr(int32(17)),
	PtrOptInt64:   ptr(int64(18)),
	PtrOptUint:    ptr(uint(19)),
	PtrOptUint8:   ptr(uint8(20)),
	PtrOptUint16:  ptr(uint16(21)),
	PtrOptUint32:  ptr(uint32(22)),
	PtrOptUint64:  ptr(uint64(23)),
	PtrOptFloat32: ptr(float32(24.1)),
	PtrOptFloat64: ptr(float64(25.1)),
	PtrOptString:  ptr("jkl"),
	PtrOptSlice:   ptr([]string{"mno", "prs"}),
	PtrOptStruct:  ptr(oneField[int]{26}),
}

func f32(x float64) float64 {
	return float64(float32(x))
}

var optStructFullValue = object(map[string]any{
	"opt_bool":        js(true),
	"opt_int":         js(1),
	"opt_int8":        js(2),
	"opt_int16":       js(-3),
	"opt_int32":       js(4),
	"opt_int64":       js(-5),
	"opt_uint":        js(6),
	"opt_uint8":       js(7),
	"opt_uint16":      js(8),
	"opt_uint32":      js(9),
	"opt_uint64":      js(10),
	"opt_float32":     js(f32(11.1)),
	"opt_float64":     js(12.1),
	"opt_string":      js("abc"),
	"opt_slice":       list(js("def"), js("ghi")),
	"opt_struct":      object(map[string]any{"field": js(13)}),
	"ptr_opt_bool":    js(false),
	"ptr_opt_int":     js(14),
	"ptr_opt_int8":    js(15),
	"ptr_opt_int16":   js(16),
	"ptr_opt_int32":   js(17),
	"ptr_opt_int64":   js(18),
	"ptr_opt_uint":    js(19),
	"ptr_opt_uint8":   js(20),
	"ptr_opt_uint16":  js(21),
	"ptr_opt_uint32":  js(22),
	"ptr_opt_uint64":  js(23),
	"ptr_opt_float32": js(f32(24.1)),
	"ptr_opt_float64": js(25.1),
	"ptr_opt_string":  js("jkl"),
	"ptr_opt_slice":   list(js("mno"), js("prs")),
	"ptr_opt_struct":  object(map[string]any{"field": js(26)}),
})

var optStructTricky = &optStruct{
	OptSlice:      []string{}, // empty but not zero value
	PtrOptBool:    ptr(false),
	PtrOptInt:     ptr(int(0)),
	PtrOptInt8:    ptr(int8(0)),
	PtrOptInt16:   ptr(int16(0)),
	PtrOptInt32:   ptr(int32(0)),
	PtrOptInt64:   ptr(int64(0)),
	PtrOptUint:    ptr(uint(0)),
	PtrOptUint8:   ptr(uint8(0)),
	PtrOptUint16:  ptr(uint16(0)),
	PtrOptUint32:  ptr(uint32(0)),
	PtrOptUint64:  ptr(uint64(0)),
	PtrOptFloat32: ptr(float32(0)),
	PtrOptFloat64: ptr(float64(0)),
	PtrOptString:  ptr(""),
	PtrOptSlice:   ptr([]string(nil)),
	PtrOptStruct:  ptr(oneField[int]{0}),
}

var optStructTrickyValue = object(map[string]any{
	"opt_slice":       list(),
	"ptr_opt_bool":    js(false),
	"ptr_opt_int":     js(0),
	"ptr_opt_int8":    js(0),
	"ptr_opt_int16":   js(0),
	"ptr_opt_int32":   js(0),
	"ptr_opt_int64":   js(0),
	"ptr_opt_uint":    js(0),
	"ptr_opt_uint8":   js(0),
	"ptr_opt_uint16":  js(0),
	"ptr_opt_uint32":  js(0),
	"ptr_opt_uint64":  js(0),
	"ptr_opt_float32": js(0),
	"ptr_opt_float64": js(0),
	"ptr_opt_string":  js(""),
	"ptr_opt_slice":   list(),
	"ptr_opt_struct":  object(map[string]any{"field": js(0)}),
})

type cleanBuildArgs struct {
	WorkspacePaths []string `tool:"The workspace paths of the source code files for which we want to manage dependencies. ITEM: Workspace path of the file to parse (e.g. google3/path/to/file)."`
}

var cleanBuildArgsSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"workspace_paths": {
			Type:        genai.TypeArray,
			Description: "The workspace paths of the source code files for which we want to manage dependencies.",
			Items: &genai.Schema{
				Type:        genai.TypeString,
				Description: "Workspace path of the file to parse (e.g. google3/path/to/file).",
			},
		},
	},
	PropertyOrdering: []string{"workspace_paths"},
	Required:         []string{"workspace_paths"},
}

type cleanBuildReply struct {
	UpdatedBuildFiles []string           `tool:"The workspace paths of the BUILD files that were updated by build_cleaner. ITEM: Workspace path of a BUILD file that was updated by build_cleaner (e.g. google3/path/to/BUILD)."`
	Issues            []*cleanBuildIssue `tool:"Any issues that occurred during the execution of individual tasks executed by build_cleaner."`
}

type cleanBuildIssue struct {
	BlazePackage string `tool:"The blaze package on which the action was executed (e.g. file/base)."`
	Action       string `tool:"The type of action executed by build_cleaner."`
	ActionLabel  string `tool:"The label of the action executed by build_cleaner."`
	Status       string `tool:"The status of the action executed by build_cleaner."`
	StatusDetail string `tool:"Details about the status of the action executed by build_cleaner."`
}

var cleanBuildReplySchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"updated_build_files": {
			Type:        genai.TypeArray,
			Description: "The workspace paths of the BUILD files that were updated by build_cleaner.",
			Items: &genai.Schema{
				Type:        genai.TypeString,
				Description: "Workspace path of a BUILD file that was updated by build_cleaner (e.g. google3/path/to/BUILD).",
			},
		},
		"issues": {
			Type:        genai.TypeArray,
			Description: "Any issues that occurred during the execution of individual tasks executed by build_cleaner.",
			Items: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"blaze_package": {
						Type:        genai.TypeString,
						Description: "The blaze package on which the action was executed (e.g. file/base).",
					},
					"action": {
						Type:        genai.TypeString,
						Description: "The type of action executed by build_cleaner.",
					},
					"action_label": {
						Type:        genai.TypeString,
						Description: "The label of the action executed by build_cleaner.",
					},
					"status": {
						Type:        genai.TypeString,
						Description: "The status of the action executed by build_cleaner.",
					},
					"status_detail": {
						Type:        genai.TypeString,
						Description: "Details about the status of the action executed by build_cleaner.",
					},
				},
				PropertyOrdering: []string{"blaze_package", "action", "action_label", "status", "status_detail"},
				Required:         []string{"blaze_package", "action", "action_label", "status", "status_detail"},
			},
		},
	},
	PropertyOrdering: []string{"updated_build_files", "issues"},
	Required:         []string{"updated_build_files", "issues"},
}

type codeSearchArgs struct {
	Query         string   `tool:"The query to run using the Code Search query language."`
	Languages     []string `tool:"#optional List of programming languages to filter the search results. When specified, we only return results from those languages."`
	Paths         []string `tool:"#optional List of regexes matching the paths of the files to search in. Only file paths matching all the regexes will be searched."`
	NextPageToken string   `tool:"#optional Token for the next page of results. If not specified, the first page is returned."`
}

var codeSearchArgsSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"query": {
			Type:        genai.TypeString,
			Description: "The query to run using the Code Search query language.",
		},
		"languages": {
			Type:        genai.TypeArray,
			Description: "List of programming languages to filter the search results. When specified, we only return results from those languages.",
			Items: &genai.Schema{
				Type: genai.TypeString,
			},
		},
		"paths": {
			Type:        genai.TypeArray,
			Description: "List of regexes matching the paths of the files to search in. Only file paths matching all the regexes will be searched.",
			Items: &genai.Schema{
				Type: genai.TypeString,
			},
		},
		"next_page_token": {
			Type:        genai.TypeString,
			Description: "Token for the next page of results. If not specified, the first page is returned.",
		},
	},
	Required:         []string{"query"},
	PropertyOrdering: []string{"query", "languages", "paths", "next_page_token"},
}

type codeSearchReply struct {
	Results               []*codeSearchMatch `tool:"#optional List of file paths that match the search query, along with matched code snippets. Maximum 5 results are returned."`
	NextPageToken         string             `tool:"#optional Token to retrieve the next page of results. Absent if there are no more results."`
	EstimatedTotalResults int                `tool:"#optional Estimated total number of results for the search query."`
	Message               string             `tool:"#optional An additional helpful informational message."`
}

type codeSearchMatch struct {
	Path     string   `tool:"#optional Workspace path of the file that matches the search query."`
	Snippets []string `tool:"#optional List of code snippets that match the search query in the file."`
}

var codeSearchReplySchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"results": {
			Type:        genai.TypeArray,
			Description: "List of file paths that match the search query, along with matched code snippets. Maximum 5 results are returned.",
			Items: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"path": {
						Type:        genai.TypeString,
						Description: "Workspace path of the file that matches the search query.",
					},
					"snippets": {
						Type:        genai.TypeArray,
						Description: "List of code snippets that match the search query in the file.",
						Items: &genai.Schema{
							Type: genai.TypeString,
						},
					},
				},
				PropertyOrdering: []string{"path", "snippets"},
			},
		},
		"next_page_token": {
			Type:        genai.TypeString,
			Description: "Token to retrieve the next page of results. Absent if there are no more results.",
		},
		"estimated_total_results": {
			Type:        genai.TypeInteger,
			Description: "Estimated total number of results for the search query.",
		},
		"message": {
			Type:        genai.TypeString,
			Description: "An additional helpful informational message.",
		},
	},
	PropertyOrdering: []string{"results", "next_page_token", "estimated_total_results", "message"},
}
