package server

import (
	"strings"
	"unicode/utf8"

	types "github.com/inference-gateway/adk/types"
)

var opaqueParamFields = map[string]bool{
	"metadata": true,
	"data":     true,
	"header":   true,
	"params":   true,
}

// normalizeParams accepts the proto field names the A2A JSON binding allows alongside their
// lowerCamelCase form (spec section 1.4) by rewriting request param keys to lowerCamelCase.
func normalizeParams(params *types.Struct) *types.Struct {
	if params == nil {
		return nil
	}
	normalized := types.Struct(camelizeKeys(*params))
	return &normalized
}

func camelizeKeys(params map[string]any) map[string]any {
	camelized := make(map[string]any, len(params))
	for key, value := range params {
		if !opaqueParamFields[key] {
			value = camelizeValue(value)
		}
		camelized[toLowerCamelCase(key)] = value
	}
	return camelized
}

func camelizeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return camelizeKeys(typed)
	case []any:
		camelized := make([]any, len(typed))
		for index, item := range typed {
			camelized[index] = camelizeValue(item)
		}
		return camelized
	default:
		return value
	}
}

func toLowerCamelCase(key string) string {
	segments := strings.Split(key, "_")
	camelized := segments[0]
	for _, segment := range segments[1:] {
		if segment == "" {
			camelized += "_"
			continue
		}
		first, size := utf8.DecodeRuneInString(segment)
		camelized += strings.ToUpper(string(first)) + segment[size:]
	}
	return camelized
}
