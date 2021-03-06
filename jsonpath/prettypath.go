package jsonpath

import (
	"github.com/zclconf/go-cty/cty"
	"bytes"
	"fmt"
	"strconv"
)

// Pretty prints a cty.Path into a string
//
// Example:
//   PrettyCtyPath(c.IndexInt(3)) == ".[3]"
func PrettyCtyPath(path cty.Path) string {
	var buf bytes.Buffer
	for _, step := range path {
		switch ts := step.(type) {
		case cty.GetAttrStep:
			fmt.Fprintf(&buf, ".%s", ts.Name)
		case cty.IndexStep:
			buf.WriteByte('[')
			key := ts.Key
			keyTy := key.Type()
			switch {
			case key.IsNull():
				buf.WriteString("null")
			case !key.IsKnown():
				buf.WriteString("(not yet known)")
			case keyTy == cty.Number:
				bf := key.AsBigFloat()
				buf.WriteString(bf.Text('g', -1))
			case keyTy == cty.String:
				buf.WriteString(strconv.Quote(key.AsString()))
			default:
				buf.WriteString("...")
			}
			buf.WriteByte(']')
		}
	}
	return buf.String()
}
