package list_params

func FilterEq(field string) string {
	return filterWithOperator(field, OperatorEq)
}

func FilterNeq(field string) string {
	return filterWithOperator(field, OperatorNeq)
}

func FilterLt(field string) string {
	return filterWithOperator(field, OperatorLt)
}

func FilterGt(field string) string {
	return filterWithOperator(field, OperatorGt)
}

func FilterLte(field string) string {
	return filterWithOperator(field, OperatorLte)
}

func FilterGte(field string) string {
	return filterWithOperator(field, OperatorGte)
}

func FilterIn(field string) string {
	return filterWithOperator(field, OperatorIn)
}

func FilterNin(field string) string {
	return filterWithOperator(field, OperatorNin)
}

func FilterLike(field string) string {
	return filterWithOperator(field, OperatorLike)
}

func filterWithOperator(field string, operator Operator) string {
	return field + operatorDelimiter + string(operator)
}
