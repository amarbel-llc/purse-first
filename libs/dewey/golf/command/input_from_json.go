package command

import (
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/dewey/bravo/collections_slice"
)

// makeInputFromJSON constructs a CommandLineInput from JSON args, walking
// the Param declarations in order to populate positional Args. This allows
// Cmd.Run(Request) commands registered via AddCmd to use PopArg and friends.
func makeInputFromJSON(raw json.RawMessage, params []Param) CommandLineInput {
	var input CommandLineInput

	if len(raw) == 0 || len(params) == 0 {
		return input
	}

	var vals map[string]json.RawMessage
	if err := json.Unmarshal(raw, &vals); err != nil {
		return input
	}

	// Walk params in declaration order, extracting string values for
	// positional args. This mirrors what RunCLI does: positional args
	// are assigned in declaration order.
	var args collections_slice.String
	for _, p := range params {
		v, ok := vals[p.paramName()]
		if !ok {
			continue
		}

		// Variadic args arrive as JSON arrays from MCP.
		if p.isVariadic() {
			var arr []string
			if err := json.Unmarshal(v, &arr); err == nil {
				args.Append(arr...)
				continue
			}
			// Fall through: if it's not an array, treat as single value.
		}

		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			// Not a string — try to use the raw JSON representation.
			s = fmt.Sprintf("%s", string(v))
		}
		args.Append(s)
	}

	input.Args = args
	return input
}
