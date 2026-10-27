package parser

import (
	"strconv"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

var ctyString = cty.String

func indexString(s hcl.TraverseIndex) string {
	k := s.Key
	if k.IsNull() || !k.IsKnown() {
		return "?"
	}
	switch k.Type() {
	case cty.String:
		return strconv.Quote(k.AsString())
	case cty.Number:
		return k.AsBigFloat().Text('f', -1)
	}
	return "?"
}
