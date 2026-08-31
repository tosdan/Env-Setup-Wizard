package domain

// LineEnding is the consistent newline sequence used by a Document.
type LineEnding string

const (
	// LineEndingLF represents Unix-style line endings.
	LineEndingLF LineEnding = "\n"
	// LineEndingCRLF represents Windows-style line endings.
	LineEndingCRLF LineEnding = "\r\n"
)

// ValueSource identifies where a resolved Variable value originated.
type ValueSource string

const (
	ValueFromTemplate ValueSource = "template"
	ValueFromExisting ValueSource = "existing"
	ValueFromUser     ValueSource = "user"
	ValueFromFixed    ValueSource = "fixed"
)

// Document preserves the ordered structure and newline format of a Template.
type Document struct {
	Nodes           []Node
	LineEnding      LineEnding
	HasFinalNewline bool
}

// Node is one ordered source line in a Document. Its implementations are
// intentionally closed to the domain package.
type Node interface {
	RawLine() string
	LineNumber() int
	isNode()
}

// Comment is a normal comment line that must be preserved in generated output.
type Comment struct {
	Raw  string
	Line int
}

func (node Comment) RawLine() string { return node.Raw }
func (node Comment) LineNumber() int { return node.Line }
func (Comment) isNode()              {}

// BlankLine is an empty or horizontal-whitespace-only source line.
type BlankLine struct {
	Raw  string
	Line int
}

func (node BlankLine) RawLine() string { return node.Raw }
func (node BlankLine) LineNumber() int { return node.Line }
func (BlankLine) isNode()              {}

// AnnotationName identifies one supported Template annotation.
type AnnotationName string

const (
	AnnotationPrompt      AnnotationName = "prompt"
	AnnotationDescription AnnotationName = "description"
	AnnotationRequired    AnnotationName = "required"
	AnnotationSecret      AnnotationName = "secret"
	AnnotationType        AnnotationName = "type"
	AnnotationOptions     AnnotationName = "options"
	AnnotationPlaceholder AnnotationName = "placeholder"
	AnnotationFixed       AnnotationName = "fixed"
	AnnotationSection     AnnotationName = "section"
)

// VariableType selects the validation and interaction semantics of a Variable.
type VariableType string

const (
	VariableTypeString VariableType = "string"
	VariableTypeInt    VariableType = "int"
	VariableTypeBool   VariableType = "bool"
	VariableTypePort   VariableType = "port"
	VariableTypeURL    VariableType = "url"
)

// Annotations contains the wizard metadata bound to a Variable.
type Annotations struct {
	Prompt      string
	Description string
	Required    bool
	Secret      bool
	Type        VariableType
	Options     []string
	Placeholder string
	Fixed       bool
}

// AnnotationLine is a distinguished comment line with its parsed name and
// optional value. It remains a Node so the writer preserves its source position.
type AnnotationLine struct {
	Name  AnnotationName
	Value string
	Raw   string
	Line  int
}

func (node AnnotationLine) RawLine() string { return node.Raw }
func (node AnnotationLine) LineNumber() int { return node.Line }
func (AnnotationLine) isNode()              {}

// Variable is a structurally valid dotenv assignment. RawValue is the exact
// text after the first equals sign, while Value is its resolved content.
// HasValue distinguishes an empty resolved value from a value not yet assigned.
type Variable struct {
	Key                string
	RawValue           string
	Value              string
	HasValue           bool
	ValueSource        ValueSource
	Annotations        Annotations
	Section            string
	ExistingValueIssue *ExistingValueIssue
	Raw                string
	Line               int
}

func (node Variable) RawLine() string { return node.Raw }
func (node Variable) LineNumber() int { return node.Line }
func (Variable) isNode()              {}
