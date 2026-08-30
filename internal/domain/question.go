package domain

// QuestionKind identifies the interaction shape required by a Question.
type QuestionKind string

const (
	QuestionKindInput   QuestionKind = "input"
	QuestionKindConfirm QuestionKind = "confirm"
	QuestionKindSelect  QuestionKind = "select"
)

// ExistingValueIssue is a presentation-safe diagnostic for an incompatible
// value loaded from Existing configuration. Message never contains a secret.
type ExistingValueIssue struct {
	Message string
}

// Question is one configurable interaction derived from a Variable.
type Question struct {
	Key                string
	Prompt             string
	Description        string
	Value              string
	HasValue           bool
	ValueSource        ValueSource
	Type               VariableType
	Kind               QuestionKind
	Required           bool
	Secret             bool
	Options            []string
	Placeholder        string
	Section            string
	ExistingValueIssue *ExistingValueIssue
}

// QuestionGroup contains all configurable Questions assigned to one Section.
// Groups and Questions retain their first-occurrence document order.
type QuestionGroup struct {
	Section   string
	Questions []Question
}
