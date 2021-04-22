package list_params

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/iancoleman/strcase"
)

type Includes struct {
	passedIncludes  []string
	customIncludes  []customIncludes
	errors          []error
	allowedIncludes []string
	fieldsSet       []interface{}
	selectedFields  []interface{}
}

const includesSeparator = "."

type customIncludesFunc func(records []interface{}) error

type customIncludes struct {
	Field string
	Func  customIncludesFunc
}

// NewIncludes returns new Includes from URL query string
func NewIncludes(query string) *Includes {
	values, err := url.ParseQuery(query)
	if err != nil {
		return nil
	}
	includes := &Includes{}
	includes.set(values)
	return includes
}

// Validate returns true if params are valid.
// Returns false with list of errors otherwise
func (self *Includes) Validate() (bool, []error) {
	for _, v := range self.passedIncludes {
		if !self.isAllowedIncludes(v) {
			self.addAllowingError(v)
		}
	}
	return len(self.errors) == 0, self.errors
}

func (self *Includes) AddIncludes(name string) {
	self.passedIncludes = append(self.passedIncludes, name)
}

// GetCustomIncludesFunctions returns list of customIncludesFunc needed to apply
func (self *Includes) GetCustomIncludesFunctions() []customIncludesFunc {
	functions := make([]customIncludesFunc, 0)
	for _, customIncludes := range self.customIncludes {
		if self.isIncluded(customIncludes.Field) {
			functions = append(functions, customIncludes.Func)
		}
	}
	return functions
}

// Allow sets list of allowed fields
func (self *Includes) Allow(fields []string) {
	self.allowedIncludes = fields
}

// AddCustomIncludes adds custom includes func for specific field
func (self *Includes) AddCustomIncludes(field string, function customIncludesFunc) {
	self.customIncludes = append(self.customIncludes, customIncludes{field, function})
}

// GetOutputFields returns fields needed to be serialized.
// Depends on passed includes
func (i *Includes) GetOutputFields() []interface{} {
	var fieldToDisplay []interface{}
	if len(i.selectedFields) > 0 {
		fieldToDisplay = getFieldsToDisplay(i.fieldsSet, i.selectedFields)
	} else {
		fieldToDisplay = i.fieldsSet
	}

	return i.getOutputFieldsRecursively(fieldToDisplay, []string{})
}

// AllowSelectFields sets all possible fields can be serialized
func (self *Includes) AllowSelectFields(fieldsSet []interface{}) {
	self.fieldsSet = fieldsSet
}

// GetPreloads returns list of needed preloads for eager loading
func (self *Includes) GetPreloads() []string {
	notCustomIncludes := self.getNotCustomIncludes()
	preloads := make([]string, len(notCustomIncludes))
	for i, v := range notCustomIncludes {
		preloads[i] = self.transformPreloading(v)
	}
	return preloads
}

func (i *Includes) SelectFields(fields []interface{}) {
	i.selectedFields = fields
}

func (i *Includes) GetSelectQuery() []string {
	if len(i.fieldsSet) == 0 {
		return []string{"*"}
	}
	return i.selectQueryRecursively(i.GetOutputFields(), []string{})
}

func (i *Includes) selectQueryRecursively(fields []interface{}, rootElements []string) []string {
	result := make([]string, 0)
	for _, field := range fields {
		if reflect.ValueOf(field).Kind() == reflect.String {
			selectField := field.(string)
			row := append(rootElements, selectField)
			result = append(result, strings.Join(row, includesSeparator))
			continue
		}

		for key, values := range field.(map[string][]interface{}) {
			newKey := strcase.ToSnake(key)
			if !i.isCustomIncludes(newKey) {
				newRootElements := append(rootElements, newKey)
				result = append(result, i.selectQueryRecursively(values, newRootElements)...)
			}
		}
	}

	return result
}

func (self *Includes) transformPreloading(field string) string {
	fields := strings.Split(field, includesSeparator)
	newField := make([]string, len(fields))
	for i, v := range fields {
		newField[i] = strcase.ToCamel(v)
	}
	return strings.Join(newField, includesSeparator)
}

func (self *Includes) getNotCustomIncludes() []string {
	notCustom := make([]string, 0)
	for _, includes := range self.passedIncludes {
		if !self.isCustomIncludes(includes) && self.isIncluded(includes) {
			notCustom = append(notCustom, includes)
		}
	}
	return notCustom
}

func (self *Includes) isCustomIncludes(field string) bool {
	for _, v := range self.customIncludes {
		if v.Field == field {
			return true
		}
	}
	return false
}

func (self *Includes) set(values url.Values) {
	list := make([]string, 0)
	fields := values["include"]

	for _, queryField := range fields {
		fields := strings.Split(queryField, queryParamDelimiter)
		for _, fieldName := range fields {
			list = append(list, fieldName)
		}
	}

	self.passedIncludes = list
}

func (self *Includes) addAllowingError(field string) {
	text := fmt.Sprintf("Including of %s in not allowed", field)
	err := errors.New(text)
	self.errors = append(self.errors, err)
}

func (self *Includes) isAllowedIncludes(field string) bool {
	for _, v := range self.allowedIncludes {
		if v == field {
			return true
		}
	}
	return false
}

func (self *Includes) isIncluded(relation string) bool {
	for _, v := range self.passedIncludes {
		if v == relation {
			return true
		}
	}
	return false
}

func getFieldsToDisplay(allowedFields []interface{}, selectedFields []interface{}) []interface{} {
	result := make([]interface{}, 0)
	for _, key := range allowedFields {
		if reflect.ValueOf(key).Kind() == reflect.String {
			if hasSelectedFieldStr(key, selectedFields) {
				result = append(result, key)
			}
			continue
		}

		allowedMap := key.(map[string][]interface{})
		if selectedMap := getAllowedSelectedMap(allowedMap, selectedFields); len(selectedMap) != 0 {
			result = append(result, selectedMap)
		}
	}

	return result
}

func getAllowedSelectedMap(allowedMap map[string][]interface{}, selectedFields []interface{}) map[string][]interface{} {
	result := make(map[string][]interface{}, 0)
	for key, value := range allowedMap {
		if innerSelectedFields := getSelectedFieldsByMapKey(key, selectedFields); innerSelectedFields != nil {
			result[key] = getFieldsToDisplay(value, innerSelectedFields)
		}
	}
	return result
}

func getSelectedFieldsByMapKey(mapKey string, selectedFields []interface{}) []interface{} {
	for _, mapField := range selectedFields {
		if reflect.ValueOf(mapField).Kind() == reflect.Map {
			originalSelectedMap := mapField.(map[string][]interface{})
			for k, v := range originalSelectedMap {
				if k == mapKey {
					return v
				}
			}
		}
	}
	return nil
}

func hasSelectedFieldStr(elem interface{}, fields []interface{}) bool {
	for _, field := range fields {
		if reflect.ValueOf(field).Kind() == reflect.String && field == elem {
			return true
		}
	}
	return false
}

func selectedFieldMap(elem interface{}, fields []interface{}) (mapKey string, mapFields []interface{}) {
	for _, field := range fields {
		if reflect.ValueOf(field).Kind() == reflect.Map && isMapHasKey(elem, field) {
			mapKey = elem.(string)
			mapFields = field.(map[string][]interface{})[mapKey]
			return
		}
	}
	return "", nil
}

func isMapHasKey(elem interface{}, mapField interface{}) bool {
	for key := range mapField.(map[string][]interface{}) {
		if key == elem {
			return true
		}
	}
	return false
}

func (self *Includes) getOutputFieldsRecursively(fieldsSet []interface{}, rootFields []string) []interface{} {
	result := make([]interface{}, 0)

	for _, name := range fieldsSet {
		if reflect.ValueOf(name).Kind() == reflect.String {
			result = append(result, name)
		} else {
			for fieldNameStr, mapFields := range name.(map[string][]interface{}) {
				var includedName string
				rootFieldsLen := len(rootFields)
				if rootFieldsLen == 0 {
					includedName = strcase.ToLowerCamel(fieldNameStr)
				} else {
					fields := make([]string, rootFieldsLen+1)
					for i, rootField := range rootFields {
						fields[i] = strcase.ToLowerCamel(rootField)
					}
					fields[rootFieldsLen] = strcase.ToLowerCamel(fieldNameStr)
					includedName = strings.Join(fields, ".")
				}

				if self.isIncluded(includedName) {
					indludedModel := map[string][]interface{}{
						fieldNameStr: self.getOutputFieldsRecursively(mapFields, append(rootFields, fieldNameStr))}
					result = append(result, indludedModel)
				}
			}
		}
	}
	return result
}
