package reflector

import (
	"errors"
	"reflect"
	"sync"
)

type (
	InspectType uint

	// Tag defines a single struct's string literal tag
	Tag struct {
		// Key is the tag key, such as json, xml, etc..
		// i.e: `json:"foo,omitempty". Here key is: "json"
		Key string

		// Name is a part of the value
		// i.e: `json:"foo,omitempty". Here name is: "foo"
		Name string

		// Options is a part of the value. It contains a slice of tag options i.e:
		// `json:"foo,omitempty". Here options is: ["omitempty"]
		Options []string
	}

	// Tags represent a set of tags from a single struct field
	Tags struct {
		tags []*Tag
	}

	// Field
	Field struct {
		flags        uint16
		Name         string
		Type         reflect.Type
		Value        reflect.Value
		Tags         Tags
		Relation     *Model // Relation represents a relation between a Field and a Model
		printNesting int
	}

	// Model
	Model struct {
		Name         string
		ModelType    reflect.Type
		Fields       []*Field
		printNesting int
	}

	// Reflector
	Reflector struct {
		currentModel  *Model
		currentField  *Field
		MethodsLookup []string
	}

	// a safe map of models that Reflector keeps as cached
	safeModelsMap struct {
		m map[reflect.Type]*Model
		l *sync.RWMutex
	}
)

const (
	ff_is_anonymous uint8 = 0
	ff_is_time      uint8 = 1
	ff_is_slice     uint8 = 2
	ff_is_struct    uint8 = 3
	ff_is_map       uint8 = 4
	ff_is_pointer   uint8 = 5
	ff_is_relation  uint8 = 6
	ff_is_interface uint8 = 7

	//go:generate stringer -type=InspectType types.go
	None InspectType = iota
	T_Map
	T_MapKey
	T_MapValue
	T_Slice
	T_SliceElem
	T_Array
	T_ArrayElem
	T_Struct
	T_StructField
	T_Inspect
)

var (
	printDebug bool = false

	cachedModels   *safeModelsMap
	visitingModels *safeModelsMap
	// SkipField can be returned from visiting functions to skip visiting
	// the value of this field. This is only valid in the following functions:
	//
	//   - StructField: skips visiting the struct value
	//
	SkipField = errors.New("skip this entry")

	errTagSyntax      = errors.New("bad syntax for struct tag pair")
	errTagKeySyntax   = errors.New("bad syntax for struct tag key")
	errTagValueSyntax = errors.New("bad syntax for struct tag value")

	errKeyNotSet      = errors.New("tag key does not exist")
	errTagNotExist    = errors.New("tag does not exist")
	errTagKeyMismatch = errors.New("mismatch between key and tag.key")
)