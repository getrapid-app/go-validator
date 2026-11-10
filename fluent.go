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

// The methods below mirror the rule constructors of the same name. See
// each constructor for details.

func (c *RuleChain) Bail() *RuleChain       { return c.Use(Bail()) }
func (c *RuleChain) Nullable() *RuleChain   { return c.Use(Nullable()) }
func (c *RuleChain) Sometimes() *RuleChain  { return c.Use(Sometimes()) }
func (c *RuleChain) AcceptZero() *RuleChain { return c.Use(AcceptZero()) }
func (c *RuleChain) Required() *RuleChain   { return c.Use(Required()) }
func (c *RuleChain) RequiredIf(other string, values ...string) *RuleChain {
	return c.Use(RequiredIf(other, values...))
}
func (c *RuleChain) RequiredUnless(other string, values ...string) *RuleChain {
	return c.Use(RequiredUnless(other, values...))
}
func (c *RuleChain) RequiredWith(others ...string) *RuleChain { return c.Use(RequiredWith(others...)) }
func (c *RuleChain) RequiredWithAll(others ...string) *RuleChain {
	return c.Use(RequiredWithAll(others...))
}
func (c *RuleChain) RequiredWithout(others ...string) *RuleChain {
	return c.Use(RequiredWithout(others...))
}
func (c *RuleChain) RequiredWithoutAll(others ...string) *RuleChain {
	return c.Use(RequiredWithoutAll(others...))
}
func (c *RuleChain) RequiredIfAccepted(other string) *RuleChain {
	return c.Use(RequiredIfAccepted(other))
}
func (c *RuleChain) RequiredIfDeclined(other string) *RuleChain {
	return c.Use(RequiredIfDeclined(other))
}
func (c *RuleChain) RequiredArrayKeys(keys ...string) *RuleChain {
	return c.Use(RequiredArrayKeys(keys...))
}
func (c *RuleChain) Filled() *RuleChain  { return c.Use(Filled()) }
func (c *RuleChain) Present() *RuleChain { return c.Use(Present()) }
func (c *RuleChain) PresentIf(other string, values ...string) *RuleChain {
	return c.Use(PresentIf(other, values...))
}
func (c *RuleChain) PresentUnless(other string, values ...string) *RuleChain {
	return c.Use(PresentUnless(other, values...))
}
func (c *RuleChain) PresentWith(others ...string) *RuleChain { return c.Use(PresentWith(others...)) }
func (c *RuleChain) PresentWithAll(others ...string) *RuleChain {
	return c.Use(PresentWithAll(others...))
}
func (c *RuleChain) Missing() *RuleChain { return c.Use(Missing()) }
func (c *RuleChain) MissingIf(other string, values ...string) *RuleChain {
	return c.Use(MissingIf(other, values...))
}
func (c *RuleChain) MissingUnless(other string, values ...string) *RuleChain {
	return c.Use(MissingUnless(other, values...))
}
func (c *RuleChain) MissingWith(others ...string) *RuleChain { return c.Use(MissingWith(others...)) }
func (c *RuleChain) MissingWithAll(others ...string) *RuleChain {
	return c.Use(MissingWithAll(others...))
}
func (c *RuleChain) Prohibited() *RuleChain { return c.Use(Prohibited()) }
func (c *RuleChain) ProhibitedIf(other string, values ...string) *RuleChain {
	return c.Use(ProhibitedIf(other, values...))
}
func (c *RuleChain) ProhibitedUnless(other string, values ...string) *RuleChain {
	return c.Use(ProhibitedUnless(other, values...))
}
func (c *RuleChain) ProhibitedIfAccepted(other string) *RuleChain {
	return c.Use(ProhibitedIfAccepted(other))
}
func (c *RuleChain) ProhibitedIfDeclined(other string) *RuleChain {
	return c.Use(ProhibitedIfDeclined(other))
}
func (c *RuleChain) Prohibits(others ...string) *RuleChain { return c.Use(Prohibits(others...)) }
func (c *RuleChain) Accepted() *RuleChain                  { return c.Use(Accepted()) }
func (c *RuleChain) AcceptedIf(other string, values ...string) *RuleChain {
	return c.Use(AcceptedIf(other, values...))
}
func (c *RuleChain) Declined() *RuleChain { return c.Use(Declined()) }
func (c *RuleChain) DeclinedIf(other string, values ...string) *RuleChain {
	return c.Use(DeclinedIf(other, values...))
}
func (c *RuleChain) Exclude() *RuleChain { return c.Use(Exclude()) }
func (c *RuleChain) ExcludeIf(other string, values ...string) *RuleChain {
	return c.Use(ExcludeIf(other, values...))
}
func (c *RuleChain) ExcludeUnless(other string, values ...string) *RuleChain {
	return c.Use(ExcludeUnless(other, values...))
}
func (c *RuleChain) ExcludeWith(other string) *RuleChain      { return c.Use(ExcludeWith(other)) }
func (c *RuleChain) ExcludeWithout(other string) *RuleChain   { return c.Use(ExcludeWithout(other)) }
func (c *RuleChain) String() *RuleChain                       { return c.Use(String()) }
func (c *RuleChain) Integer() *RuleChain                      { return c.Use(Integer()) }
func (c *RuleChain) IntegerStrict() *RuleChain                { return c.Use(IntegerStrict()) }
func (c *RuleChain) Numeric() *RuleChain                      { return c.Use(Numeric()) }
func (c *RuleChain) NumericStrict() *RuleChain                { return c.Use(NumericStrict()) }
func (c *RuleChain) Boolean() *RuleChain                      { return c.Use(Boolean()) }
func (c *RuleChain) BooleanStrict() *RuleChain                { return c.Use(BooleanStrict()) }
func (c *RuleChain) Array(keys ...string) *RuleChain          { return c.Use(Array(keys...)) }
func (c *RuleChain) List() *RuleChain                         { return c.Use(ListRule()) }
func (c *RuleChain) JSON() *RuleChain                         { return c.Use(JSON()) }
func (c *RuleChain) Alpha(ascii ...bool) *RuleChain           { return c.Use(Alpha(ascii...)) }
func (c *RuleChain) AlphaDash(ascii ...bool) *RuleChain       { return c.Use(AlphaDash(ascii...)) }
func (c *RuleChain) AlphaNum(ascii ...bool) *RuleChain        { return c.Use(AlphaNum(ascii...)) }
func (c *RuleChain) ASCII() *RuleChain                        { return c.Use(ASCII()) }
func (c *RuleChain) Lowercase() *RuleChain                    { return c.Use(Lowercase()) }
func (c *RuleChain) Uppercase() *RuleChain                    { return c.Use(Uppercase()) }
func (c *RuleChain) StartsWith(prefixes ...string) *RuleChain { return c.Use(StartsWith(prefixes...)) }
func (c *RuleChain) EndsWith(suffixes ...string) *RuleChain   { return c.Use(EndsWith(suffixes...)) }
func (c *RuleChain) DoesntStartWith(prefixes ...string) *RuleChain {
	return c.Use(DoesntStartWith(prefixes...))
}
func (c *RuleChain) DoesntEndWith(suffixes ...string) *RuleChain {
	return c.Use(DoesntEndWith(suffixes...))
}
func (c *RuleChain) Regex(pattern string) *RuleChain       { return c.Use(Regex(pattern)) }
func (c *RuleChain) NotRegex(pattern string) *RuleChain    { return c.Use(NotRegex(pattern)) }
func (c *RuleChain) Email(styles ...EmailStyle) *RuleChain { return c.Use(Email(styles...)) }
func (c *RuleChain) URL(protocols ...string) *RuleChain    { return c.Use(URL(protocols...)) }
func (c *RuleChain) ActiveURL() *RuleChain                 { return c.Use(ActiveURL()) }
func (c *RuleChain) UUID() *RuleChain                      { return c.Use(UUID()) }
func (c *RuleChain) ULID() *RuleChain                      { return c.Use(ULID()) }
func (c *RuleChain) IP() *RuleChain                        { return c.Use(IP()) }
func (c *RuleChain) IPv4() *RuleChain                      { return c.Use(IPv4()) }
func (c *RuleChain) IPv6() *RuleChain                      { return c.Use(IPv6()) }
func (c *RuleChain) MACAddress() *RuleChain                { return c.Use(MACAddress()) }
func (c *RuleChain) HexColor() *RuleChain                  { return c.Use(HexColor()) }
func (c *RuleChain) Size(n float64) *RuleChain             { return c.Use(Size(n)) }
func (c *RuleChain) Min(n float64) *RuleChain              { return c.Use(Min(n)) }
func (c *RuleChain) Max(n float64) *RuleChain              { return c.Use(Max(n)) }
func (c *RuleChain) Between(lo, hi float64) *RuleChain     { return c.Use(Between(lo, hi)) }
func (c *RuleChain) GreaterThan(other string) *RuleChain   { return c.Use(GreaterThan(other)) }
func (c *RuleChain) GreaterThanOrEqual(other string) *RuleChain {
	return c.Use(GreaterThanOrEqual(other))
}
func (c *RuleChain) LessThan(other string) *RuleChain        { return c.Use(LessThan(other)) }
func (c *RuleChain) LessThanOrEqual(other string) *RuleChain { return c.Use(LessThanOrEqual(other)) }
func (c *RuleChain) Digits(n int) *RuleChain                 { return c.Use(Digits(n)) }
func (c *RuleChain) DigitsBetween(lo, hi int) *RuleChain     { return c.Use(DigitsBetween(lo, hi)) }
func (c *RuleChain) MinDigits(n int) *RuleChain              { return c.Use(MinDigits(n)) }
func (c *RuleChain) MaxDigits(n int) *RuleChain              { return c.Use(MaxDigits(n)) }
func (c *RuleChain) Decimal(lo, hi int) *RuleChain           { return c.Use(Decimal(lo, hi)) }
func (c *RuleChain) MultipleOf(n string) *RuleChain          { return c.Use(MultipleOf(n)) }
func (c *RuleChain) Date() *RuleChain                        { return c.Use(Date()) }
func (c *RuleChain) DateFormat(formats ...string) *RuleChain { return c.Use(DateFormat(formats...)) }
func (c *RuleChain) After(date string) *RuleChain            { return c.Use(After(date)) }
func (c *RuleChain) AfterOrEqual(date string) *RuleChain     { return c.Use(AfterOrEqual(date)) }
func (c *RuleChain) Before(date string) *RuleChain           { return c.Use(Before(date)) }
func (c *RuleChain) BeforeOrEqual(date string) *RuleChain    { return c.Use(BeforeOrEqual(date)) }
func (c *RuleChain) DateEquals(date string) *RuleChain       { return c.Use(DateEquals(date)) }
func (c *RuleChain) Timezone() *RuleChain                    { return c.Use(Timezone()) }
func (c *RuleChain) Same(other string) *RuleChain            { return c.Use(Same(other)) }
func (c *RuleChain) Different(other string) *RuleChain       { return c.Use(Different(other)) }
func (c *RuleChain) Confirmed(field ...string) *RuleChain    { return c.Use(Confirmed(field...)) }
func (c *RuleChain) In(values ...string) *RuleChain          { return c.Use(In(values...)) }
func (c *RuleChain) NotIn(values ...string) *RuleChain       { return c.Use(NotIn(values...)) }
func (c *RuleChain) InArray(other string) *RuleChain         { return c.Use(InArray(other)) }
func (c *RuleChain) InArrayKeys(keys ...string) *RuleChain   { return c.Use(InArrayKeys(keys...)) }
func (c *RuleChain) Distinct() *RuleChain                    { return c.Use(Distinct()) }
func (c *RuleChain) Contains(values ...string) *RuleChain    { return c.Use(Contains(values...)) }
func (c *RuleChain) DoesntContain(values ...string) *RuleChain {
	return c.Use(DoesntContain(values...))
}
func (c *RuleChain) File() *RuleChain                      { return c.Use(FileRule()) }
func (c *RuleChain) Image(allowSVG ...bool) *RuleChain     { return c.Use(Image(allowSVG...)) }
func (c *RuleChain) Mimes(extensions ...string) *RuleChain { return c.Use(Mimes(extensions...)) }
func (c *RuleChain) MimeTypes(types ...string) *RuleChain  { return c.Use(MimeTypes(types...)) }
func (c *RuleChain) Extensions(extensions ...string) *RuleChain {
	return c.Use(Extensions(extensions...))
}
func (c *RuleChain) Exists(table, column string) *RuleChain {
	return c.Use(Exists(table).Column(column))
}
func (c *RuleChain) Unique(table, column string) *RuleChain {
	return c.Use(Unique(table).Column(column))
}
func (c *RuleChain) CurrentPassword() *RuleChain { return c.Use(CurrentPassword()) }

// AnyOf appends a rule that passes when any of the rule sets passes.
func (c *RuleChain) AnyOf(sets ...any) *RuleChain { return c.Use(AnyOf(sets...)) }
