package list_params

import (
	"strings"
)

type Operator string

const (
	OperatorEq   = Operator("eq")
	OperatorNeq  = Operator("neq")
	OperatorLt   = Operator("lt")
	OperatorGt   = Operator("gt")
	OperatorLte  = Operator("lte")
	OperatorGte  = Operator("gte")
	OperatorIn   = Operator("in")
	OperatorNin  = Operator("nin")
	OperatorLike = Operator("like")
)

const operatorDelimiter = ":"

type operation func(field string, values []string) (string, []interface{})

var (
	operations = map[Operator]operation{
		OperatorEq: func(field string, values []string) (string, []interface{}) {
			return expressionTemplate(field, "OR", "=", values)
		},
		OperatorNeq: func(field string, values []string) (string, []interface{}) {
			return expressionTemplate(field, "OR", "!=", values)
		},
		OperatorLt: func(field string, values []string) (string, []interface{}) {
			return expressionTemplate(field, "AND", "<", values)
		},
		OperatorGt: func(field string, values []string) (string, []interface{}) {
			return expressionTemplate(field, "AND", ">", values)
		},
		OperatorLte: func(field string, values []string) (string, []interface{}) {
			return expressionTemplate(field, "AND", "<=", values)
		},
		OperatorGte: func(field string, values []string) (string, []interface{}) {
			return expressionTemplate(field, "AND", ">=", values)
		},
		OperatorIn: func(field string, values []string) (string, []interface{}) {
			args := make([]interface{}, len(values))
			for i, v := range values {
				args[i] = v
			}
			return field + " IN (?)", args
		},
		OperatorNin: func(field string, values []string) (string, []interface{}) {
			args := make([]interface{}, len(values))
			for i, v := range values {
				args[i] = v
			}
			return field + " NOT IN (?)", args
		},
		OperatorLike: func(field string, values []string) (string, []interface{}) {
			if len(values) == 0 {
				return "", []interface{}{}
			}
			return field + " LIKE ?", []interface{}{"%" + values[0] + "%"}
		},
	}
	knownOperators = map[string]Operator{
		"eq":   OperatorEq,
		"neq":  OperatorNeq,
		"lt":   OperatorLt,
		"gt":   OperatorGt,
		"lte":  OperatorLte,
		"gte":  OperatorGte,
		"in":   OperatorIn,
		"nin":  OperatorNin,
		"like": OperatorLike,
	}
)

func expressionTemplate(field, connector, operator string, values []string) (string, []interface{}) {
	if len(values) == 0 {
		return "", nil
	}
	template := field + " " + operator + " ?"
	if len(values) == 1 {
		return template, []interface{}{values[0]}
	}
	templates := make([]string, len(values))
	for i := 0; i < len(values); i++ {
		templates[i] = template
	}

	res := strings.Join(templates, " "+connector+" ")
	args := make([]interface{}, len(values))
	for i, v := range values {
		args[i] = v
	}

	return res, args
}
