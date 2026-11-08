package validator

// RuleChain is the fluent rule builder returned by Chain. It implements
// RuleSet, so it can be used directly as a value in Rules.
//
//	validator.Rules{
//		"name":  validator.Chain().Required().String().Max(255),
//		"even":  validator.Chain().Required().Rule("even"),      // custom rule by name
//		"code":  validator.Chain().Required().Use(myCodeRule),   // custom Rule value
//	}
type RuleChain struct {
	steps []func(*Registry) (Rule, error)
}

// Chain starts an empty fluent rule chain.
func Chain() *RuleChain { return &RuleChain{} }

// Build implements RuleSet.
func (c *RuleChain) Build(r *Registry) ([]Rule, error) {
	out := make([]Rule, 0, len(c.steps))
	for _, step := range c.steps {
		rule, err := step(r)
		if err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, nil
}

// Use appends rule values, typically custom rules.
func (c *RuleChain) Use(rules ...Rule) *RuleChain {
	for _, rule := range rules {
		rule := rule
		c.steps = append(c.steps, func(*Registry) (Rule, error) { return rule, nil })
	}
	return c
}

// Rule appends a rule looked up by name in the validator's registry, the
// same way the string syntax does: Rule("between", "1", "10").
func (c *RuleChain) Rule(name string, params ...string) *RuleChain {
	c.steps = append(c.steps, func(r *Registry) (Rule, error) { return r.Make(name, params...) })
	return c
}

func (c *RuleChain) Bail() *RuleChain      { return c.Use(Bail()) }
func (c *RuleChain) Nullable() *RuleChain  { return c.Use(Nullable()) }
func (c *RuleChain) Sometimes() *RuleChain { return c.Use(Sometimes()) }
func (c *RuleChain) Required() *RuleChain  { return c.Use(Required()) }
func (c *RuleChain) RequiredIf(other string, values ...string) *RuleChain {
	return c.Use(RequiredIf(other, values...))
}
func (c *RuleChain) String() *RuleChain                { return c.Use(String()) }
func (c *RuleChain) Integer() *RuleChain               { return c.Use(Integer()) }
func (c *RuleChain) Numeric() *RuleChain               { return c.Use(Numeric()) }
func (c *RuleChain) Boolean() *RuleChain               { return c.Use(Boolean()) }
func (c *RuleChain) Array() *RuleChain                 { return c.Use(Array()) }
func (c *RuleChain) Email() *RuleChain                 { return c.Use(Email()) }
func (c *RuleChain) Min(n float64) *RuleChain          { return c.Use(Min(n)) }
func (c *RuleChain) Max(n float64) *RuleChain          { return c.Use(Max(n)) }
func (c *RuleChain) Between(lo, hi float64) *RuleChain { return c.Use(Between(lo, hi)) }
func (c *RuleChain) In(values ...string) *RuleChain    { return c.Use(In(values...)) }
func (c *RuleChain) Same(other string) *RuleChain      { return c.Use(Same(other)) }
