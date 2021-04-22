package list_params

import (
	"reflect"
	"strings"
)

const defaultFieldsDelimiter = "."

// Fields represents structure of passed fields list with nested models
type Fields struct {
	PropName     string
	List         []string
	NestedFields []Fields
}

func InterfaceArrayToFields(values []interface{}) Fields {
	return interfaceArrayToFieldsRecursively("", values)
}

func StringsArrayToFields(values []string) Fields {
	fields := Fields{PropName: "", List: make([]string, 0), NestedFields: make([]Fields, 0)}
	for _, value := range values {
		addFieldToFields(&fields, value)
	}
	return fields
}

func (f *Fields) ToInterfaceArray() []interface{} {
	result := make([]interface{}, 0)

	for _, v := range f.List {
		result = append(result, v)
	}

	nestedMap := make(map[string][]interface{})
	for _, v := range f.NestedFields {
		nestedMap[v.PropName] = v.ToInterfaceArray()
	}
	result = append(result, nestedMap)

	return result
}

func addFieldToFields(model *Fields, field string) {
	splitedValue := strings.Split(field, defaultFieldsDelimiter)
	if len(splitedValue) == 1 {
		model.List = append(model.List, field)
	} else {
		addNestedFieldToFields(model, splitedValue)
	}
}

func addNestedFieldToFields(model *Fields, splitedField []string) {
	for _, v := range model.NestedFields {
		if v.PropName == splitedField[0] {
			addFieldToFields(&v, strings.Join(splitedField[1:], defaultFieldsDelimiter))
			return
		}
	}
	newFields := Fields{PropName: splitedField[0], List: make([]string, 0), NestedFields: make([]Fields, 0)}
	addFieldToFields(&newFields, strings.Join(splitedField[1:], defaultFieldsDelimiter))
	model.NestedFields = append(model.NestedFields, newFields)
}

func interfaceArrayToFieldsRecursively(propName string, values []interface{}) Fields {
	newFields := Fields{PropName: propName, List: make([]string, 0), NestedFields: make([]Fields, 0)}

	for _, field := range values {
		if reflect.TypeOf(field).Kind() == reflect.String {
			newFields.List = append(newFields.List, field.(string))
		} else {
			for k, mapFields := range field.(map[string][]interface{}) {
				newFields.NestedFields = append(newFields.NestedFields, interfaceArrayToFieldsRecursively(k, mapFields))
			}
		}
	}

	return newFields
}
