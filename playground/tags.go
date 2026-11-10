package playground

// tagInfo describes how a go-playground tag is used as a pg.<tag> rule.
type tagInfo struct {
	param   paramUse
	typed   bool // the message depends on the value: string, numeric, array or date
	listSep bool // parameters are a list, which go-playground separates with spaces
}

type paramUse uint8

const (
	paramNone     paramUse = iota // "pg.email"
	paramOptional                 // "pg.gt" (a date after now) or "pg.gt:5"
	paramRequired                 // "pg.len:6"
)

// tags are the go-playground validations that work on a single value. The
// ones left out need the rest of the struct (eqfield, required_if, ...),
// are presence checks that Laravel's rules already cover (required,
// omitempty, isdefault), or read the server's file system from user input
// (file, dir, image, mimetype, uds_exists).
var tags = map[string]tagInfo{
	// Sizes and comparisons. Strings compare their length in characters,
	// numbers their value, lists and maps their number of items.
	"len": {param: paramRequired, typed: true},
	"min": {param: paramRequired, typed: true},
	"max": {param: paramRequired, typed: true},
	"eq":  {param: paramOptional, typed: true},
	"ne":  {param: paramOptional, typed: true},
	"lt":  {param: paramOptional, typed: true},
	"lte": {param: paramOptional, typed: true},
	"gt":  {param: paramOptional, typed: true},
	"gte": {param: paramOptional, typed: true},

	"eq_ignore_case": {param: paramRequired},
	"ne_ignore_case": {param: paramRequired},

	// Lists of allowed or forbidden values.
	"oneof":    {param: paramRequired, listSep: true},
	"oneofci":  {param: paramRequired, listSep: true},
	"noneof":   {param: paramRequired, listSep: true},
	"noneofci": {param: paramRequired, listSep: true},
	"unique":   {param: paramOptional},

	// Strings.
	"alpha":           {},
	"alphaspace":      {},
	"alphanum":        {},
	"alphanumspace":   {},
	"alphaunicode":    {},
	"alphanumunicode": {},
	"ascii":           {},
	"printascii":      {},
	"multibyte":       {},
	"lowercase":       {},
	"uppercase":       {},
	"boolean":         {},
	"numeric":         {},
	"number":          {},
	"hexadecimal":     {},
	"contains":        {param: paramRequired},
	"containsany":     {param: paramRequired},
	"containsrune":    {param: paramRequired},
	"excludes":        {param: paramRequired},
	"excludesall":     {param: paramRequired},
	"excludesrune":    {param: paramRequired},
	"startswith":      {param: paramRequired},
	"endswith":        {param: paramRequired},
	"startsnotwith":   {param: paramRequired},
	"endsnotwith":     {param: paramRequired},
	"html":            {},
	"html_encoded":    {},
	"url_encoded":     {},
	"json":            {},
	"jwt":             {},
	"datauri":         {},
	"base32":          {},
	"base64":          {},
	"base64url":       {},
	"base64rawurl":    {},

	// Colors.
	"hexcolor": {},
	"rgb":      {},
	"rgba":     {},
	"hsl":      {},
	"hsla":     {},
	"cmyk":     {},
	"iscolor":  {},

	// Web and network.
	"email":             {},
	"e164":              {},
	"url":               {},
	"http_url":          {},
	"https_url":         {},
	"uri":               {},
	"origin":            {},
	"urn_rfc2141":       {},
	"urn_rfc8141":       {},
	"filepath":          {},
	"dirpath":           {},
	"ip":                {},
	"ipv4":              {},
	"ipv6":              {},
	"cidr":              {},
	"cidrv4":            {},
	"cidrv6":            {},
	"ip_addr":           {},
	"ip4_addr":          {},
	"ip6_addr":          {},
	"tcp_addr":          {},
	"tcp4_addr":         {},
	"tcp6_addr":         {},
	"udp_addr":          {},
	"udp4_addr":         {},
	"udp6_addr":         {},
	"unix_addr":         {},
	"mac":               {},
	"hostname":          {},
	"hostname_rfc1123":  {},
	"hostname_label":    {},
	"hostname_port":     {},
	"fqdn":              {},
	"port":              {},
	"dns_rfc1035_label": {},

	// Identifiers and hashes.
	"uuid":          {},
	"uuid3":         {},
	"uuid4":         {},
	"uuid5":         {},
	"uuid7":         {},
	"uuid_rfc4122":  {},
	"uuid3_rfc4122": {},
	"uuid4_rfc4122": {},
	"uuid5_rfc4122": {},
	"ulid":          {},
	"md4":           {},
	"md5":           {},
	"sha256":        {},
	"sha384":        {},
	"sha512":        {},
	"ripemd128":     {},
	"ripemd160":     {},
	"tiger128":      {},
	"tiger160":      {},
	"tiger192":      {},
	"mongodb":       {},
	"cve":           {},
	"semver":        {},
	"cron":          {},
	"spicedb":       {param: paramOptional},

	// Money, finance and government numbers.
	"credit_card":       {},
	"luhn_checksum":     {},
	"bic":               {},
	"bic_iso_9362_2014": {},
	"iso4217":           {},
	"iso4217_numeric":   {},
	"btc_addr":          {},
	"btc_addr_bech32":   {},
	"eth_addr":          {},
	"eth_addr_checksum": {},
	"isbn":              {},
	"isbn10":            {},
	"isbn13":            {},
	"issn":              {},
	"ssn":               {},
	"ein":               {},

	// Places, languages and time.
	"latitude":                   {},
	"longitude":                  {},
	"iso3166_1_alpha2":           {},
	"iso3166_1_alpha2_eu":        {},
	"iso3166_1_alpha3":           {},
	"iso3166_1_alpha3_eu":        {},
	"iso3166_1_alpha_numeric":    {},
	"iso3166_1_alpha_numeric_eu": {},
	"country_code":               {},
	"eu_country_code":            {},
	"iso3166_2":                  {},
	"postcode_iso3166_alpha2":    {param: paramRequired},
	"bcp47_language_tag":         {},
	"bcp47_strict_language_tag":  {},
	"timezone":                   {},
	"datetime":                   {param: paramRequired},
	"duration":                   {},

	"mongodb_connection_string": {},
}
