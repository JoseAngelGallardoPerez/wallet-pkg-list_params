package list_params

import (
	"fmt"
	"log"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/jinzhu/inflection"
)

const AscDirection = "ASC"
const DescDirection = "DESC"
const queryParamDelimiter = ","

const DefaultPageNumber = 1
const DefaultPageSize = 20

const (
	JoinLeft  = Join("LEFT")
	JoinRight = Join("RIGHT")
	JoinInner = Join("INNER")
)

const tableNameFuncName = "TableName"
const sqlTableFieldDelimiter = "."

type Join string

type FieldOperatorPair struct {
	Field    string // Field of current or nested model. For nested model should be used format: [relationship name].[field name]
	Operator Operator
}

type FilterListParameter struct {
	FieldOperatorPair
	Values []string
}

type PaginationListParameter struct {
	PageNumber uint32
	PageSize   uint32
}

type SortingListParameter struct {
	Field     string
	Direction string
}

type ListParams struct {
	Sortings   []SortingListParameter
	Filters    []FilterListParameter
	Includes   *Includes // Fields in model. Example: likes, author, author.likes
	Pagination PaginationListParameter

	ObjectType        reflect.Type
	allowedListParams allowedListParams
	customFilters     []customFilter
	customIncludes    []customIncludes
	customSortings    []customSoting
	errors            []error
	joins             []join
	groupBy           *string
}

type join struct {
	tableName   string
	onStatement string
	joinType    Join
}

type customFilter struct {
	Field string
	Func  customFilterFunc
}

type customSoting struct {
	Field string
	Func  customSortingFunc
}

type allowedListParams struct {
	Sortings   []string
	Filters    []FieldOperatorPair
	Pagination bool
}

type customFilterFunc func(inputValues []string, params *ListParams) (
	dbConditionPart string, dbValues interface{})

type customSortingFunc func(direction string, params *ListParams) (orderByPart string, err error)

// NewListParamsFromQuery creates new ListParams from passed url query
// and type of serialized object
func NewListParamsFromQuery(query string, object interface{}) *ListParams {
	unescapedQuery, err := url.PathUnescape(query)
	if err != nil {
		log.Printf("List params. Error in unesaped query: %v\n", err)
	}
	values, err := url.ParseQuery(unescapedQuery)

	listParams := NewListParams()
	if reflect.Indirect(reflect.ValueOf(object)).IsValid() {
		listParams.ObjectType = reflect.TypeOf(object)
	} else {
		listParams.ObjectType = reflect.TypeOf(reflect.ValueOf(object).Elem())
	}

	if err != nil {
		listParams.addError("Query format is invalid")
		return listParams
	}

	listParams.setSortingParams(values)
	listParams.setFilters(values)
	listParams.Includes = NewIncludes(query)
	listParams.setPagination(values)

	return listParams
}

// NewListParams returns new empty ListParams
func NewListParams() *ListParams {
	listParams := ListParams{allowedListParams: newAllowedListParams(),
		customFilters:  make([]customFilter, 0),
		customSortings: make([]customSoting, 0),
		errors:         make([]error, 0),
		joins:          make([]join, 0),
		Includes:       NewIncludes(""),
		groupBy:        nil,
	}
	return &listParams
}

// Validate checks if all passed options was allowed
func (params *ListParams) Validate() (bool, []error) {
	for _, v := range params.Sortings {
		if !params.isAllowedSorting(v.Field) {
			params.addSortingError(v.Field)
		}
	}
	for _, v := range params.Filters {
		if !params.isAllowedFilter(v.Field, v.Operator) {
			params.addFilterError(v.Field, v.Operator)
		}
	}
	if ok, errors := params.Includes.Validate(); !ok {
		params.addErrors(errors)
	}
	if !params.allowedListParams.Pagination {
		pagination := params.Pagination
		if pagination.PageNumber != DefaultPageNumber || (pagination.PageSize != DefaultPageSize && pagination.PageSize != 0) {
			params.addPaginationError()
		}
	}

	return len(params.errors) == 0, params.errors
}

// AddLeftJoin adds needed joinType for complex options
func (params *ListParams) AddLeftJoin(tableName string, onStatement string) {
	params.AddJoin(tableName, onStatement, JoinLeft)
}

// AddRightJoin adds needed joinType for complex options
func (params *ListParams) AddRightJoin(tableName string, onStatement string) {
	params.AddJoin(tableName, onStatement, JoinRight)
}

// AddInnerJoin adds needed joinType for complex options
func (params *ListParams) AddInnerJoin(tableName string, onStatement string) {
	params.AddJoin(tableName, onStatement, JoinInner)
}

// AddJoin adds needed joinType for complex options
func (params *ListParams) AddJoin(tableName string, onStatement string, joinType Join) {
	j := join{tableName, onStatement, joinType}
	if !params.isJoinExist(j) {
		params.joins = append(params.joins, j)
	}
}

// AddFilter adds filter manually
// Can be used in custom filter function
func (params *ListParams) AddFilter(field string, values []string, operator ...Operator) {
	op := OperatorEq
	if len(operator) != 0 {
		op = operator[0]
	} //field, op,values
	pair := FieldOperatorPair{Field: field, Operator: op}
	params.Filters = append(params.Filters, FilterListParameter{pair, values})
}

// AddCustomFilter adds custom filter.
// If passed field is overridden by custom filter
// then custom filter will be used by calling customFilterFunc
func (params *ListParams) AddCustomFilter(field string, function customFilterFunc) {
	params.customFilters = append(params.customFilters, customFilter{field, function})
}

// AddCustomIncludes adds custom includes. Custom includes overrides usual includes
func (params *ListParams) AddCustomIncludes(field string, function customIncludesFunc) {
	params.Includes.AddCustomIncludes(field, function)
}

// AddCustomSortings adds custom sortings. Overrides usual sorting
func (params *ListParams) AddCustomSortings(field string, function customSortingFunc) {
	params.customSortings = append(params.customSortings, customSoting{field, function})
}

// AllowSelectFields sets allowed list of fields. Than fields can be returned
// depends on includes for Serializers
func (params *ListParams) AllowSelectFields(fieldsSet []interface{}) {
	params.Includes.AllowSelectFields(fieldsSet)
}

// AllowPagination allows pagination. Params will be invalid
// if pagination is not allowed and params for pagination passed
func (params *ListParams) AllowPagination() {
	params.allowedListParams.Pagination = true
}

// AllowFilters allows filters. Needed to be valid
func (params *ListParams) AllowFilters(fields []string) {
	pairs := make([]FieldOperatorPair, len(fields))
	for i, fieldWithOperator := range fields {
		field, operator := params.parseField(fieldWithOperator)
		pairs[i] = FieldOperatorPair{Field: field, Operator: operator}
	}
	params.allowedListParams.Filters = pairs
}

// AllowIncludes allows includes. Needed to be valid
func (params *ListParams) AllowIncludes(fields []string) {
	params.Includes.Allow(fields)
}

// AllowSortings allows sorting. Needed to be valid
func (params *ListParams) AllowSortings(fields []string) {
	params.allowedListParams.Sortings = fields
}

func (params *ListParams) SetGroupBy(groupBy string) {
	params.groupBy = &groupBy
}

func (params *ListParams) GetGroupBy() *string {
	return params.groupBy
}

// GetOrderByString returns valid SQL string for ORDER BY statement
func (params *ListParams) GetOrderByString() string {
	orderByParts := make([]string, 0)

	for _, sorting := range params.Sortings {
		if orderPart, err := params.getOrderByString(&sorting); err != nil {
		} else {
			orderByParts = append(orderByParts, orderPart)
		}
	}
	return strings.Join(orderByParts, ",")
}

// GetCustomIncludesFunctions calls GetCustomIncludesFunctions to Includes
func (params *ListParams) GetCustomIncludesFunctions() []customIncludesFunc {
	return params.Includes.GetCustomIncludesFunctions()
}

// GetPreloads
func (params *ListParams) GetPreloads() []string {
	return params.Includes.GetPreloads()
}

// GetOutputFields calls GetOutputFields for Includes
func (params *ListParams) GetOutputFields() []interface{} {
	return params.Includes.GetOutputFields()
}

// GetJoinCondition returns SQL string with joins
func (params *ListParams) GetJoinCondition() string {
	joinParts := make([]string, len(params.joins))
	for i, joinProps := range params.joins {
		joinParts[i] = fmt.Sprintf("%s JOIN %s ON %s", joinProps.joinType, joinProps.tableName, joinProps.onStatement)
	}
	return strings.Join(joinParts, " ")
}

// GetWhereCondition returns sql string with params for where statement
func (params *ListParams) GetWhereCondition() (string, []interface{}) {
	filterStrs := make([]string, 0)
	arguments := make([]interface{}, 0, len(params.Filters))

	for _, filter := range params.Filters {
		var conditionPart string
		if custom := params.getCustomFilter(filter.Field); custom != nil {
			//custom filter func may use multiple placeholders and return multiple arguments
			//we have to check if it returned slice of arguments and append each the argument
			//with unique index
			//for example consider the following condition "(accounts.user_id = ? OR cards.user_id = ?)
			conditionP, customFilterArgs := params.getConditionPartFromCustomFilter(custom, filter.Values)
			conditionPart = conditionP
			if reflect.TypeOf(customFilterArgs).Kind() == reflect.Slice {
				args := reflect.ValueOf(customFilterArgs)
				for j := 0; j < args.Len(); j++ {
					arguments = append(arguments, args.Index(j).Interface())
				}
			} else {
				arguments = append(arguments, customFilterArgs)
			}
		} else {
			conditionP, args := params.GetConditionPartFromUsualFilter(&filter)
			conditionPart = conditionP
			arguments = append(arguments, args)
		}

		filterStrs = append(filterStrs, conditionPart)
	}
	return strings.Join(filterStrs, " AND "), arguments
}

// GetLimit returns limit can
func (params *ListParams) GetLimit() uint32 {
	return params.Pagination.PageSize
}

// GetOffset returns offset
func (params *ListParams) GetOffset() uint32 {
	return params.Pagination.PageSize * (params.Pagination.PageNumber - 1)
}

// GetConditionPartFromUsualFilter returns where condition string with params
func (params *ListParams) GetConditionPartFromUsualFilter(filter *FilterListParameter) (string, interface{}) {
	if operation, ok := operations[filter.Operator]; ok {
		transformName := params.transformName(filter.Field)
		if len(strings.Split(transformName, sqlTableFieldDelimiter)) == 1 {
			transformName = params.addTablePrefix(transformName)
		}
		return operation(transformName, filter.Values)
	}
	if len(filter.Values) == 1 {
		conditionStr := fmt.Sprintf("%s = ?", params.transformName(filter.Field))
		return conditionStr, filter.Values[0]
	}
	conditionStr := fmt.Sprintf("%s IN (?)", params.transformName(filter.Field))
	return conditionStr, filter.Values
}

// SelectFields receives list of fields in format as for AllowSelectFields method
// Sets fields to select from db and returned by GetOutputFields method
func (params *ListParams) SelectFields(fields []interface{}) {
	params.Includes.SelectFields(fields)
}

func (params *ListParams) GetSelectQuery() []string {
	return params.Includes.GetSelectQuery()
}

// isJoinExist checks if given joinType exist in array of joins
func (params *ListParams) isJoinExist(j join) bool {
	for _, v := range params.joins {
		if v.tableName == j.tableName && v.onStatement == j.onStatement && v.joinType == j.joinType {
			return true
		}
	}
	return false
}

// isDescDirection returns true if direction is DESC
func (sortingParams *SortingListParameter) isDescDirection() bool {
	return sortingParams.Direction == DescDirection
}

// getOrderByString returns SQL string for ORDER BY
func (params *ListParams) getOrderByString(sortingParam *SortingListParameter) (string, error) {
	if custom := params.getCustomSorting(sortingParam.Field); custom != nil {
		return custom.Func(sortingParam.Direction, params)
	}
	transformName := params.transformName(sortingParam.Field)
	if len(strings.Split(transformName, sqlTableFieldDelimiter)) == 1 {
		transformName = params.addTablePrefix(transformName)
	}

	return fmt.Sprintf("%s %s", transformName, sortingParam.Direction), nil
}

func newAllowedListParams() allowedListParams {
	return allowedListParams{make([]string, 0), make([]FieldOperatorPair, 0), false}
}

func (params *ListParams) getConditionPartFromCustomFilter(
	filter *customFilter, inputValues []string) (string, interface{}) {
	return filter.Func(inputValues, params)
}

func (params *ListParams) getCustomFilter(field string) *customFilter {
	for _, filter := range params.customFilters {
		if filter.Field == field {
			return &filter
		}
	}
	return nil
}

func (params *ListParams) getCustomSorting(field string) *customSoting {
	for _, v := range params.customSortings {
		if v.Field == field {
			return &v
		}
	}
	return nil
}

// setSortingParams sets array of sorting params taken from url.Values.
// Supports many sort params divided by comma
func (params *ListParams) setSortingParams(values url.Values) {
	sortingFields := values["sort"]
	list := make([]SortingListParameter, 0)

	for _, sortingQueryParam := range sortingFields {
		fields := strings.Split(sortingQueryParam, queryParamDelimiter)
		for _, field := range fields {
			sortingParameter := SortingListParameter{}
			if (string)(field[0]) == "-" {
				sortingParameter.Direction = DescDirection
				sortingParameter.Field = field[1:]
			} else {
				sortingParameter.Direction = AscDirection
				sortingParameter.Field = field
			}
			list = append(list, sortingParameter)
		}
	}

	params.Sortings = list
}

func (params *ListParams) addTablePrefix(field string) string {
	var tableName string
	if method, ok := reflect.PtrTo(params.ObjectType).MethodByName(tableNameFuncName); ok {
		result := method.Func.Call([]reflect.Value{reflect.New(params.ObjectType)})
		tableName = result[0].Interface().(string)
	} else {
		pluralName := inflection.Plural(params.ObjectType.Name())
		tableName = strcase.ToSnake(pluralName)

	}
	return strings.Join([]string{tableName, field}, sqlTableFieldDelimiter)
}

// setPagination takes first page[number] and page[size] params
// or applies default values
func (params *ListParams) setPagination(values url.Values) {
	var number32, size32 uint32
	if len(values["page[number]"]) == 0 {
		number32 = DefaultPageNumber
	} else {
		number := values["page[number]"][0]
		number64, _ := strconv.ParseUint(number, 10, 32)
		number32 = uint32(number64)
	}

	if len(values["page[size]"]) == 0 {
		size32 = DefaultPageSize
	} else {
		size := values["page[size]"][0]
		size64, _ := strconv.ParseUint(size, 10, 32)
		size32 = uint32(size64)
	}

	params.Pagination = PaginationListParameter{number32, size32}
}

func (params *ListParams) setFilters(values url.Values) {
	list := make([]FilterListParameter, 0)
	pattern := regexp.MustCompile(`filter\[(.*)\]`)
	for k := range values {
		submatches := pattern.FindStringSubmatch(k)
		if len(submatches) < 2 {
			continue
		}
		values := strings.Split(values[submatches[0]][0], queryParamDelimiter)
		field, operator := params.parseField(submatches[1])
		pair := FieldOperatorPair{Field: field, Operator: operator}
		parameter := FilterListParameter{FieldOperatorPair: pair, Values: values}
		list = append(list, parameter)
	}

	params.Filters = list
}

func (params *ListParams) parseField(field string) (fieldName string, operator Operator) {
	fieldAndOperator := strings.Split(field, operatorDelimiter)
	if len(fieldAndOperator) == 0 {
		return field, OperatorEq
	}

	operator = OperatorEq
	if len(fieldAndOperator) == 1 {
		return fieldAndOperator[0], OperatorEq
	}

	if op, ok := knownOperators[fieldAndOperator[1]]; ok {
		fieldName = fieldAndOperator[0]
		operator = op
	}
	return
}

func (params *ListParams) addPaginationError() {
	params.addError("Pagination is not allowed")
}

func (params *ListParams) addFilterError(field string, operator Operator) {
	params.addError(fmt.Sprintf("Filter %s is not allowed with operator %s", field, operator))
}

func (params *ListParams) addIncludesError(field string) {
	params.addError(fmt.Sprintf("Including of %s in not allowed", field))
}

func (params *ListParams) addSortingError(field string) {
	params.addError(fmt.Sprintf("Sorting by %s in not allowed", field))
}

func (params *ListParams) addError(text string) {
	params.errors = append(params.errors, NewErrorString(text))
}

func (params *ListParams) addErrors(errors []error) {
	for _, err := range errors {
		params.errors = append(params.errors, NewErrorString(err.Error()))
	}
}

func (params *ListParams) isAllowedFilter(field string, operator Operator) bool {
	for _, v := range params.allowedListParams.Filters {
		if v.Field == field && v.Operator == operator {
			return true
		}
	}
	return false
}

func (params *ListParams) isAllowedSorting(field string) bool {
	for _, v := range params.allowedListParams.Sortings {
		if v == field {
			return true
		}
	}
	return false
}

// transformName transforms name to snake case
func (params *ListParams) transformName(presentedName string) string {
	fieldsCount := params.ObjectType.NumField()
	for i := 0; i < fieldsCount; i++ {
		field := params.ObjectType.Field(i)
		if field.Tag.Get("json") == presentedName {
			if dbTag, ok := field.Tag.Lookup("db"); ok {
				return dbTag
			}
			return strcase.ToSnake(field.Name)
		}
	}
	return strcase.ToSnake(presentedName)
}
