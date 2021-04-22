package list_params

func NewErrorString(text string) error {
	return &ErrorString{text}
}

type ErrorString struct {
	S string `json:"error"`
}

func (e *ErrorString) Error() string {
	return e.S
}