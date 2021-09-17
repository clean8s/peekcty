# JSONPath for the cty Go library

`jsonpathcty` lets you iterate over `cty` datastructures using JSONPath syntax.

Note: [cty](https://github.com/zclconf/go-cty/) is the serialization / typesystem library
for Go powering HCL, Terraform, zclconf.

## Example

```go
import "github.com/clean8s/jsonpathcty"

func demo() {
  name := jsonpathcty.MustNewPath("$.Name").Apply(someObj)
}
```

## Implementation

It's based on [spyzhov/ajson](https://github.com/spyzhov/ajson), by replacing
`ajson.Node` operations with the corresponding `cty` operations.