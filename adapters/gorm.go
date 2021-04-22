package adapters

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/jinzhu/gorm"

	"github.com/Confialink/wallet-pkg-list_params"
)

// Gorm adapter for github.com/jinzhu/gorm
type Gorm struct {
	db *gorm.DB
}

func NewGorm(db *gorm.DB) *Gorm {
	return &Gorm{db}
}

// LoadList loads records from db.
// Pass slice, not adress and list params.
func (adapter *Gorm) LoadList(recordsPtr interface{}, params *list_params.ListParams, table string) error {
	str, arguments := params.GetWhereCondition()
	query := adapter.db.Where(str, arguments...)

	query = query.Order(params.GetOrderByString())

	if params.GetLimit() != 0 {
		query = query.Limit(params.GetLimit())
	}
	query = query.Offset(params.GetOffset())

	query = query.Joins(params.GetJoinCondition())

	for _, preloadName := range params.GetPreloads() {
		query = query.Preload(preloadName)
	}

	selectQuery := transformSelectQuery(params.GetSelectQuery(), params.ObjectType, table)
	query = query.Select(selectQuery)
	if err := query.Table(table).Find(recordsPtr).Error; err != nil {
		return err
	}

	slice := reflect.ValueOf(recordsPtr).Elem()
	recordsSlice := make([]interface{}, slice.Len())
	for i := 0; i < slice.Len(); i++ {
		recordsSlice[i] = slice.Index(i).Interface()
	}

	for _, customIncludesFunc := range params.GetCustomIncludesFunctions() {
		if err := customIncludesFunc(recordsSlice); err != nil {
			return err
		}
	}

	return nil
}

func transformSelectQuery(paramsQuery []string, modelType reflect.Type, table string) []string {
	if paramsQuery[0] == "*" {
		return paramsQuery
	}
	result := make([]string, len(paramsQuery))
	for i, v := range paramsQuery {
		result[i] = transformRootField(v, modelType, table)
	}
	return result
}

func transformRootField(field string, modelType reflect.Type, tableName string) string {
	splitField := strings.Split(field, ".")
	if len(splitField) == 1 {
		return strings.Join([]string{tableName, transformField(field, modelType)}, ".")
	}
	return transformField(field, modelType)
}

func transformField(field string, modelType reflect.Type) string {
	splitedField := strings.Split(field, ".")
	if len(splitedField) == 1 {
		return transformSingleField(field, modelType)
	}
	return transformNestedField(splitedField, modelType)
}

func transformSingleField(field string, modelType reflect.Type) string {
	if columnName := getColumn(field, modelType); columnName != "" {
		return columnName
	}

	return gorm.ToDBName(field)
}

func transformNestedField(splitedField []string, modelType reflect.Type) string {
	structFieldName := strcase.ToCamel(splitedField[0])
	nestedStruct, ok := modelType.FieldByName(structFieldName)
	if !ok {
		panic(fmt.Errorf("Can not find nested struct %s", structFieldName))
	}
	prefix := splitedField[0] + "s"
	postfix := transformField(strings.Join(splitedField[1:], "."), nestedStruct.Type)
	resultRow := []string{prefix, postfix}
	return strings.Join(resultRow, ".")
}

func getColumn(fieldName string, modelType reflect.Type) string {
	field, ok := modelType.FieldByName(fieldName)
	if !ok {
		panic(fmt.Errorf("Field %s can not be found", fieldName))
	}

	gormTag := field.Tag.Get("gorm")
	for _, option := range strings.Split(gormTag, ";") {
		optionParts := strings.Split(option, ":")
		if optionParts[0] == "column" {
			return optionParts[1]
		}
	}

	return ""
}
