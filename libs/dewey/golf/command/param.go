package command

import (
	"github.com/amarbel-llc/purse-first/libs/dewey/0/interfaces"
	"github.com/amarbel-llc/purse-first/libs/dewey/charlie/values"
)

// Completer is an iterator that yields Completion values for shell
// tab-completion and MCP enum hints. Using an iterator instead of
// returning a map allows streaming large completion sets without
// loading everything into memory.
type ParamCompleter = interfaces.Seq[Completion]

// Param is the sealed interface for command parameters. Concrete types
// are Flag[V], Arg[V], ArrayFlag, and ObjectFlag. External packages
// define new value types (V) freely but cannot add new structural kinds.
//
// This sealing is a lever that may need to open later — if it does,
// generators would also need to become open/extensible.
type Param interface {
	paramName() string
	paramDescription() string
	paramRequired() bool
	paramDefault() any
	jsonSchemaType() string
	enumValues() []string
	isPositional() bool // true for Arg, false for Flag/ArrayFlag/ObjectFlag
	isVariadic() bool   // true for variadic Arg (consumes all remaining positional args)
	paramShort() rune   // short flag rune; 0 for positional args
	isParam()
}

// schemaTypeOf returns the JSON Schema type for a FlagValue type parameter.
func schemaTypeOf[V interfaces.FlagValue]() string {
	var zero V
	switch any(zero).(type) {
	case *values.Int:
		return "integer"
	case *values.Bool:
		return "boolean"
	default:
		return "string"
	}
}

// Flag is a named CLI flag (--name / -n), also an MCP schema property.
type Flag[V interfaces.FlagValue] struct {
	Name        string
	Description string
	Required    bool
	EnumValues  []string
	Short       rune
	Default     any
	Completer   ParamCompleter
}

func (f Flag[V]) flagCompleter() ParamCompleter { return f.Completer }
func (f Flag[V]) paramName() string             { return f.Name }
func (f Flag[V]) paramDescription() string      { return f.Description }
func (f Flag[V]) paramRequired() bool           { return f.Required }
func (f Flag[V]) paramDefault() any             { return f.Default }
func (f Flag[V]) enumValues() []string          { return f.EnumValues }
func (f Flag[V]) jsonSchemaType() string        { return schemaTypeOf[V]() }
func (f Flag[V]) isPositional() bool            { return false }
func (f Flag[V]) isVariadic() bool              { return false }
func (f Flag[V]) paramShort() rune              { return f.Short }
func (f Flag[V]) isParam()                      {}

// Arg is a positional CLI argument, also an MCP schema property.
type Arg[V interfaces.FlagValue] struct {
	Name        string
	Description string
	Required    bool
	EnumValues  []string
	Variadic    bool // consumes all remaining positional args; must be last
}

func (a Arg[V]) paramName() string        { return a.Name }
func (a Arg[V]) paramDescription() string { return a.Description }
func (a Arg[V]) paramRequired() bool      { return a.Required }
func (a Arg[V]) paramDefault() any        { return nil }
func (a Arg[V]) enumValues() []string     { return a.EnumValues }
func (a Arg[V]) jsonSchemaType() string   { return schemaTypeOf[V]() }
func (a Arg[V]) isPositional() bool       { return true }
func (a Arg[V]) isVariadic() bool         { return a.Variadic }
func (a Arg[V]) paramShort() rune         { return 0 }
func (a Arg[V]) isParam()                 {}

// ArrayFlag is a repeated/array flag with nested item schema.
type ArrayFlag struct {
	Name        string
	Short       rune
	Description string
	Required    bool
	Items       []Param
}

func (a ArrayFlag) paramName() string        { return a.Name }
func (a ArrayFlag) paramDescription() string { return a.Description }
func (a ArrayFlag) paramRequired() bool      { return a.Required }
func (a ArrayFlag) paramDefault() any        { return nil }
func (a ArrayFlag) jsonSchemaType() string   { return "array" }
func (a ArrayFlag) enumValues() []string     { return nil }
func (a ArrayFlag) isPositional() bool       { return false }
func (a ArrayFlag) isVariadic() bool         { return false }
func (a ArrayFlag) paramShort() rune         { return a.Short }
func (a ArrayFlag) isParam()                 {}

// ObjectFlag is a freeform JSON object flag.
type ObjectFlag struct {
	Name        string
	Description string
	Required    bool
}

func (o ObjectFlag) paramName() string        { return o.Name }
func (o ObjectFlag) paramDescription() string { return o.Description }
func (o ObjectFlag) paramRequired() bool      { return o.Required }
func (o ObjectFlag) paramDefault() any        { return nil }
func (o ObjectFlag) jsonSchemaType() string   { return "object" }
func (o ObjectFlag) enumValues() []string     { return nil }
func (o ObjectFlag) isPositional() bool       { return false }
func (o ObjectFlag) isVariadic() bool         { return false }
func (o ObjectFlag) paramShort() rune         { return 0 }
func (o ObjectFlag) isParam()                 {}

// Concrete aliases for common param types.
type (
	StringFlag = Flag[*values.String]
	IntFlag    = Flag[*values.Int]
	BoolFlag   = Flag[*values.Bool]

	StringArg = Arg[*values.String]
	IntArg    = Arg[*values.Int]
)
