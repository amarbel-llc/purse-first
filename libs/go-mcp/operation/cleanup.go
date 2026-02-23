package operation

func (c *ctx) After(fn func() error) {
	c.afters = append(c.afters, fn)
}

func (c *ctx) Must(fn func() error) {
	c.musts = append(c.musts, fn)
}

func (c *ctx) runAfter() {
	for i := len(c.afters) - 1; i >= 0; i-- {
		_ = callSafe(c.afters[i])
	}
}

func (c *ctx) runMust() {
	for i := len(c.musts) - 1; i >= 0; i-- {
		if err := callSafe(c.musts[i]); err != nil {
			c.event.MustErrors = append(c.event.MustErrors, err)
			if c.outcome == Success {
				c.outcome = Failure
			}
		}
	}
}
