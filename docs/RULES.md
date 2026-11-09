# Rules

Every rule works in all three syntaxes. The string form goes in `[]string{...}`, pipe strings and struct tags. The fluent form is a method on `validator.Chain()` and, with the same name, a constructor such as `validator.Max(255)`.

Placeholders like `:other` take field names. Wildcards in them (`items.*.type`) resolve to the same item as the field being validated.

## Modifiers

| String | Fluent | Meaning |
|---|---|---|
| `bail` | `Bail()` | Stop at the field's first failure |
| `nullable` | `Nullable()` | `nil` skips the other rules |
| `sometimes` | `Sometimes()` | Only validate when present |
| `accept_zero` | `AcceptZero()` | `0` counts as a value for `required` and other presence rules |
| `exclude` | `Exclude()` | Skip the field |
| `exclude_if:field,v1,...` | `ExcludeIf(field, v...)`, `ExcludeIfFunc(fn)` | Skip when another field matches |
| `exclude_unless:field,v1,...` | `ExcludeUnless(field, v...)` | Skip unless another field matches |
| `exclude_with:field` | `ExcludeWith(field)` | Skip when another field is present |
| `exclude_without:field` | `ExcludeWithout(field)` | Skip when another field is missing |

## Presence

| String | Fluent |
|---|---|
| `required` | `Required()` |
| `required_if:field,v1,...` | `RequiredIf(field, v...)`, `RequiredIfFunc(fn)` |
| `required_unless:field,v1,...` | `RequiredUnless(field, v...)` |
| `required_with:f1,...` | `RequiredWith(f...)` |
| `required_with_all:f1,...` | `RequiredWithAll(f...)` |
| `required_without:f1,...` | `RequiredWithout(f...)` |
| `required_without_all:f1,...` | `RequiredWithoutAll(f...)` |
| `required_if_accepted:field` | `RequiredIfAccepted(field)` |
| `required_if_declined:field` | `RequiredIfDeclined(field)` |
| `required_array_keys:k1,...` | `RequiredArrayKeys(k...)` |
| `filled` | `Filled()` |
| `present`, `present_if`, `present_unless`, `present_with`, `present_with_all` | `Present()`, `PresentIf`, `PresentUnless`, `PresentWith`, `PresentWithAll` |
| `missing`, `missing_if`, `missing_unless`, `missing_with`, `missing_with_all` | `Missing()`, `MissingIf`, `MissingUnless`, `MissingWith`, `MissingWithAll` |
| `prohibited`, `prohibited_if`, `prohibited_unless` | `Prohibited()`, `ProhibitedIf`, `ProhibitedUnless`, `ProhibitedIfFunc(fn)` |
| `prohibited_if_accepted:field`, `prohibited_if_declined:field` | `ProhibitedIfAccepted`, `ProhibitedIfDeclined` |
| `prohibits:f1,...` | `Prohibits(f...)` |
| `accepted`, `accepted_if:field,v1,...` | `Accepted()`, `AcceptedIf(field, v...)` |
| `declined`, `declined_if:field,v1,...` | `Declined()`, `DeclinedIf(field, v...)` |

## Types

| String | Fluent |
|---|---|
| `string` | `String()` |
| `integer`, `integer:strict` | `Integer()`, `IntegerStrict()` |
| `numeric`, `numeric:strict` | `Numeric()`, `NumericStrict()` |
| `boolean`, `boolean:strict` | `Boolean()`, `BooleanStrict()` |
| `array`, `array:k1,...` | `Array(k...)` |
| `list` | `List()` on chains, `ListRule()` as a constructor |
| `json` | `JSON()` |
| (object only) | `Enum[T](values...)`, `ValidEnum[T]()` |

## Strings and formats

| String | Fluent |
|---|---|
| `alpha`, `alpha:ascii` | `Alpha(ascii...)` |
| `alpha_dash`, `alpha_num` | `AlphaDash(ascii...)`, `AlphaNum(ascii...)` |
| `ascii`, `lowercase`, `uppercase` | `ASCII()`, `Lowercase()`, `Uppercase()` |
| `starts_with:a,...`, `ends_with:a,...` | `StartsWith(a...)`, `EndsWith(a...)` |
| `doesnt_start_with:a,...`, `doesnt_end_with:a,...` | `DoesntStartWith(a...)`, `DoesntEndWith(a...)` |
| `regex:/pattern/flags`, `not_regex:...` | `Regex(p)`, `NotRegex(p)`, `RegexOf(re)` |
| `email`, `email:rfc,strict,dns,spoof,filter,filter_unicode` | `Email(styles...)` |
| `url`, `url:http,https` | `URL(protocols...)` |
| `active_url` | `ActiveURL()` |
| `uuid`, `uuid:4` | `UUID()`, `UUIDVersion(4)` |
| `ulid` | `ULID()` |
| `ip`, `ipv4`, `ipv6` | `IP()`, `IPv4()`, `IPv6()` |
| `mac_address` | `MACAddress()` |
| `hex_color` | `HexColor()` |

## Numbers and sizes

Size rules measure numbers by value, strings by characters (by value with `numeric`), lists and maps by item count and files in kilobytes.

| String | Fluent |
|---|---|
| `size:n`, `min:n`, `max:n`, `between:a,b` | `Size(n)`, `Min(n)`, `Max(n)`, `Between(a, b)` |
| `gt:field`, `gte:field`, `lt:field`, `lte:field` (a field or a number) | `GreaterThan`, `GreaterThanOrEqual`, `LessThan`, `LessThanOrEqual` |
| `digits:n`, `digits_between:a,b` | `Digits(n)`, `DigitsBetween(a, b)` |
| `min_digits:n`, `max_digits:n` | `MinDigits(n)`, `MaxDigits(n)` |
| `decimal:n`, `decimal:min,max` | `Decimal(min, max)` |
| `multiple_of:n` | `MultipleOf("n")` |

## Dates

`date` accepts `time.Time` and strings in any of `validator.DateLayouts`. The `after` family takes a date, `now`, `today`, `tomorrow`, `yesterday` or another field's name.

| String | Fluent |
|---|---|
| `date` | `Date()` |
| `date_format:Y-m-d` or `date_format:2006-01-02` | `DateFormat(formats...)` |
| `after:x`, `after_or_equal:x` | `After(x)`, `AfterOrEqual(x)` |
| `before:x`, `before_or_equal:x` | `Before(x)`, `BeforeOrEqual(x)` |
| `date_equals:x` | `DateEquals(x)` |
| `timezone` | `Timezone()` |

## Comparing and lists

| String | Fluent |
|---|---|
| `same:field`, `different:field` | `Same(f)`, `Different(f)` |
| `confirmed`, `confirmed:other_field` | `Confirmed(other...)` |
| `in:a,...`, `not_in:a,...` | `In(a...)`, `NotIn(a...)` |
| `in_array:other.*` | `InArray(other)` |
| `in_array_keys:k1,...` | `InArrayKeys(k...)` |
| `distinct`, `distinct:strict`, `distinct:ignore_case` | `Distinct()`, `DistinctStrict()`, `DistinctIgnoreCase()` |
| `contains:a,...`, `doesnt_contain:a,...` | `Contains(a...)`, `DoesntContain(a...)` |
| (object only) | `AnyOf(sets...)`, `When(cond, rules, otherwise...)` |

## Files

Values implementing `validator.File`, or `*multipart.FileHeader`, which is converted automatically.

| String | Fluent |
|---|---|
| `file` | `File()` on chains, `FileRule()` as a constructor |
| `image`, `image:allow_svg` | `Image(allowSVG...)` |
| `mimes:jpg,png` (by content) | `Mimes(ext...)` |
| `mimetypes:image/*` | `MimeTypes(types...)` |
| `extensions:jpg,png` (by name) | `Extensions(ext...)` |
| `dimensions:min_width=100,ratio=3/2` | `Dimensions().MinWidth(100).Ratio(1.5)` |

## Database and passwords

| String | Fluent |
|---|---|
| `exists:table,column` | `Exists(table).Column(c).Where(col, v)` |
| `unique:table,column,ignoreID,idColumn` | `Unique(table).Column(c).Ignore(id, "id")` |
| `current_password` | `CurrentPassword()` |
| (object only) | `Password().Min(12).Letters().MixedCase().Numbers().Symbols().Uncompromised(checker)` |

`exists` and `unique` need `validator.WithPresenceVerifier(...)`. `current_password` needs `validator.WithCurrentPassword(...)`.

## Messages that are not rules

These keys are translated and overridden like rule messages:

| Key | English |
|---|---|
| `invalid` | The :attribute field is invalid. (no message found) |
| `http.invalid` | The given data was invalid. |
| `http.more_error` | :message (and :count more error) |
| `http.more_errors` | :message (and :count more errors) |
| `http.unauthorized` | This action is unauthorized. |
| `http.server_error` | Server Error |

## Not ported

- `can`: authorization belongs in the request's `Authorize` hook.
- `uploaded`: upload errors surface when the transport decodes the request.
