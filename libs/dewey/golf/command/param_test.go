package command

import (
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/charlie/values"
)

func TestFlagImplementsParam(t *testing.T) {
	var _ Param = Flag[*values.String]{}
	var _ Param = Flag[*values.Int]{}
	var _ Param = Flag[*values.Bool]{}
}

func TestArgImplementsParam(t *testing.T) {
	var _ Param = Arg[*values.String]{}
	var _ Param = Arg[*values.Int]{}
}

func TestArrayFlagImplementsParam(t *testing.T) {
	var _ Param = ArrayFlag{}
}

func TestObjectFlagImplementsParam(t *testing.T) {
	var _ Param = ObjectFlag{}
}

func TestFlagJSONSchemaType(t *testing.T) {
	tests := []struct {
		name string
		p    Param
		want string
	}{
		{"string flag", StringFlag{Name: "path"}, "string"},
		{"int flag", IntFlag{Name: "count"}, "integer"},
		{"bool flag", BoolFlag{Name: "verbose"}, "boolean"},
		{"array flag", ArrayFlag{Name: "paths"}, "array"},
		{"object flag", ObjectFlag{Name: "args"}, "object"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.jsonSchemaType(); got != tt.want {
				t.Errorf("jsonSchemaType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestArgJSONSchemaType(t *testing.T) {
	p := StringArg{Name: "repo-id"}
	if got := p.jsonSchemaType(); got != "string" {
		t.Errorf("jsonSchemaType() = %q, want %q", got, "string")
	}

	p2 := IntArg{Name: "count"}
	if got := p2.jsonSchemaType(); got != "integer" {
		t.Errorf("jsonSchemaType() = %q, want %q", got, "integer")
	}
}

func TestParamName(t *testing.T) {
	f := StringFlag{Name: "repo_path"}
	if got := f.paramName(); got != "repo_path" {
		t.Errorf("paramName() = %q, want %q", got, "repo_path")
	}

	a := StringArg{Name: "file"}
	if got := a.paramName(); got != "file" {
		t.Errorf("paramName() = %q, want %q", got, "file")
	}
}

func TestParamEnumValues(t *testing.T) {
	f := StringFlag{
		Name:       "format",
		EnumValues: []string{"json", "text", "tap"},
	}
	ev := f.enumValues()
	if len(ev) != 3 {
		t.Fatalf("enumValues() len = %d, want 3", len(ev))
	}
	if ev[0] != "json" || ev[1] != "text" || ev[2] != "tap" {
		t.Errorf("enumValues() = %v, want [json text tap]", ev)
	}

	// No enum
	f2 := StringFlag{Name: "path"}
	if ev := f2.enumValues(); ev != nil {
		t.Errorf("enumValues() = %v, want nil", ev)
	}
}

func TestFlagDefault(t *testing.T) {
	f := IntFlag{Name: "port", Default: 8080}
	if got := f.paramDefault(); got != 8080 {
		t.Errorf("paramDefault() = %v, want 8080", got)
	}
}

func TestArgNoDefault(t *testing.T) {
	a := StringArg{Name: "file"}
	if got := a.paramDefault(); got != nil {
		t.Errorf("paramDefault() = %v, want nil", got)
	}
}

func TestConcreteAliasesAreUsable(t *testing.T) {
	// Verify aliases work for construction without explicit type params.
	_ = StringFlag{Name: "path", Description: "File path", Required: true, Short: 'p'}
	_ = IntFlag{Name: "count", Description: "Number of items", Default: 10}
	_ = BoolFlag{Name: "verbose", Description: "Verbose output", Short: 'v'}
	_ = StringArg{Name: "repo-id", Description: "Repository ID", Required: true}
	_ = IntArg{Name: "line", Description: "Line number", Required: true}
}
