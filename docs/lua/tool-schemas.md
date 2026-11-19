# tool argument schemas

model tools use JSON Schema draft 2020-12 to describe and validate arguments. the root must declare `"type": "object"`. schemas are checked at registration; arguments are checked before approval and again after editing.

```json
{
  "type": "object",
  "properties": {
    "command": { "type": "string", "minLength": 1, "maxLength": 4096 },
    "timeout": { "type": "integer", "minimum": 1, "maximum": 300 }
  },
  "required": ["command"],
  "additionalProperties": false
}
```

validation does not execute a tool, grant approval, apply defaults or change arguments. a schema annotation such as `readOnly` does not change the tool's approval policy. validation errors do not include argument values.

## supported keywords

| group | keywords |
| --- | --- |
| dialect | `$schema` with `https://json-schema.org/draft/2020-12/schema` |
| types and values | `type`, `enum`, `const` |
| objects | `properties`, `required`, `additionalProperties`, `patternProperties`, `propertyNames`, `dependentRequired`, `dependentSchemas`, `unevaluatedProperties`, `minProperties`, `maxProperties` |
| arrays | `items`, `prefixItems`, `contains`, `minContains`, `maxContains`, `uniqueItems`, `unevaluatedItems`, `minItems`, `maxItems` |
| strings | `minLength`, `maxLength`, `pattern` |
| numbers | `minimum`, `maximum`, `exclusiveMinimum`, `exclusiveMaximum`, `multipleOf` |
| composition | `allOf`, `anyOf`, `oneOf`, `not`, `if`, `then`, `else` |
| annotations | `title`, `description`, `$comment`, `default`, `examples`, `deprecated`, `readOnly`, `writeOnly` |

subschemas may be objects or booleans. property names and annotation data may contain arbitrary JSON keys. the keyword restrictions apply to schema locations, not to a property's name or an example's contents.

unknown keywords are rejected. `$ref`, `$dynamicRef`, `$defs`, `$id`, anchors, `format` and content keywords are not supported. no external schema is loaded. an omitted `$schema` selects draft 2020-12; other dialects are rejected.

patterns use Go's RE2 syntax. lookaround and backreferences are unavailable. numbers use double precision, matching Lua. integer-valued numbers outside `-9007199254740991` through `9007199254740991` are rejected in schemas and arguments, including decimal and exponent notation. use strings for larger identifiers and exact decimal values. fractional numbers retain double precision rounding.

## limits

| limit | schema | arguments |
| --- | --- | --- |
| encoded JSON | 32 kib | 32 kib |
| depth | 16 | 16 |
| nodes, including object keys | 512 | 4,096 |

both inputs reject duplicate object keys, invalid UTF-8, unpaired Unicode surrogate escapes, trailing JSON values and nonfinite numbers. escaped equivalents count as the same key.

regex source is limited to 1,024 bytes per pattern and 4,096 bytes across a schema. expanded regex programs are limited to 4,096 instructions per pattern and 8,192 across the schema. counted repetitions contribute their expanded instruction count.

combined limits also apply before validation:

- schema nodes multiplied by argument nodes must not exceed 262,144.
- schema weight multiplied by argument weight must not exceed 8,388,608. schema weight is its node count plus expanded regex instructions. argument weight is its node count plus decoded string and key bytes.

a schema and its arguments can fit their individual limits yet exceed the combined work limit. simplify repeated branches, shorten arguments or split the tool's input into bounded pages. a work-limit failure rejects the call before the validator runs.

these bounds control work and memory; they are not a wall-clock deadline. schema validation runs synchronously inside its calling worker. user-installed Lua remains trusted code with operating-system access; the schema validator is not a sandbox.

## implementation checks

`internal/modeltool.Compile` creates an immutable compiled schema. `Schema.Validate` accepts encoded arguments and returns an error without echoing their contents. compiled schemas support concurrent validation. registration must retain the compiled schema with its tool definition so pending calls validate against the implementation selected for their approval.

run the behavior checks and benchmarks with:

```sh
go test -race ./internal/modeltool
go test ./internal/modeltool -run '^$' -bench BenchmarkSchemaValidation -benchmem
```

see [managed services](services.md) for plugin processes and network requests, and [surfaces](surfaces.md) for building argument editors and result views.
