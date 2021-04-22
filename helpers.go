package list_params

import (
	"fmt"
	"strings"
)

func ConvertBoolFilterValue(inputValues []string) interface{} {
	var result string
	if inputValues[0] == "true" {
		result = "1"
	} else {
		result = "0"
	}
	return result
}

func BoolFilter(fieldName string) func([]string, *ListParams) (string, interface{}) {
	return func(inputValues []string, _ *ListParams) (string, interface{}) {
		return fmt.Sprintf("%s = ?", fieldName), ConvertBoolFilterValue(inputValues)
	}
}

func DateFromFilter(fieldName string) func([]string, *ListParams) (string, interface{}) {
	return func(inputValues []string, _ *ListParams) (string, interface{}) {
		return fmt.Sprintf("%s >= ?", fieldName), inputValues[0]
	}
}

func DateToFilter(fieldName string) func([]string, *ListParams) (string, interface{}) {
	return func(inputValues []string, _ *ListParams) (string, interface{}) {
		return fmt.Sprintf("%s < ?", fieldName), inputValues[0]
	}
}

// ContainsFilter is helper for queries format: fieldName LIKE %value%
func ContainsFilter(fieldName string) func([]string, *ListParams) (string, interface{}) {
	return func(inputValues []string, _ *ListParams) (string, interface{}) {
		conditions := make([]string, len(inputValues))
		values := make([]string, len(inputValues))
		for i, inputValue := range inputValues {
			conditions[i] = fmt.Sprintf("%s LIKE ?", fieldName)
			values[i] = fmt.Sprintf("%%%s%%", inputValue)
		}
		return fmt.Sprintf("(%s)", strings.Join(conditions, " OR ")), values
	}
}
