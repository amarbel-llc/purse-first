package command

import (
	"github.com/amarbel-llc/purse-first/libs/dewey/0/interfaces"
	"github.com/amarbel-llc/purse-first/libs/dewey/charlie/values"
)

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
	isParam()
}

// param is the shared base for generic param types.
type param[V interfaces.FlagValue] struct {
	Name        string
	Description string
	Required    bool
	EnumValues  []string
}

func (p param[V]) paramName() string        { return p.Name }
func (p param[V]) paramDescription() string  { return p.Description }
func (p param[V]) paramRequired() bool       { return p.Required }
func (p param[V]) paramDefault() any         { return nil }
func (p param[V]) enumValues() []string      { return p.EnumValues }
func (p param[V]) isParam()                  {}

func (p param[V]) jsonSchemaType() string {
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
	param[V]
	Short     rune
	Default   any
	Completer func() map[string]string
}

func (f Flag[V]) paramDefault() any { return f.Default }

// Arg is a positional CLI argument, also an MCP schema property.
type Arg[V interfaces.FlagValue] struct {
	param[V]
	Variadic bool // consumes all remaining positional args; must be last
}

// ArrayFlag is a repeated/array flag with nested item schema.
type ArrayFlag struct {
	Name        string
	Short       rune
	Description string
	Required    bool
	Items       []Param
}

func (a ArrayFlag) paramName() string        { return a.Name }
func (a ArrayFlag) paramDescription() string  { return a.Description }
func (a ArrayFlag) paramRequired() bool       { return a.Required }
func (a ArrayFlag) paramDefault() any         { return nil }
func (a ArrayFlag) jsonSchemaType() string    { return "array" }
func (a ArrayFlag) enumValues() []string      { return nil }
func (a ArrayFlag) isParam()                  {}

// ObjectFlag is a freeform JSON object flag.
type ObjectFlag struct {
	Name        string
	Description string
	Required    bool
}

func (o ObjectFlag) paramName() string        { return o.Name }
func (o ObjectFlag) paramDescription() string  { return o.Description }
func (o ObjectFlag) paramRequired() bool       { return o.Required }
func (o ObjectFlag) paramDefault() any         { return nil }
func (o ObjectFlag) jsonSchemaType() string    { return "object" }
func (o ObjectFlag) enumValues() []string      { return nil }
func (o ObjectFlag) isParam()                  {}

// Concrete aliases for common param types.
type (
	StringFlag = Flag[*values.String]
	IntFlag    = Flag[*values.Int]
	BoolFlag   = Flag[*values.Bool]

	StringArg = Arg[*values.String]
	IntArg    = Arg[*values.Int]
)
