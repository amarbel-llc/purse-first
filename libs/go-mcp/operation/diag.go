package operation

import "runtime"

func (c *ctx) DiagSet(key string, value any) {
	if c.extras == nil {
		c.extras = make(map[string]any)
	}
	c.extras[key] = value
}

func (c *ctx) DiagHelper() {
	if c.helpers == nil {
		c.helpers = make(map[string]struct{})
	}
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			c.helpers[fn.Name()] = struct{}{}
		}
	}
}
