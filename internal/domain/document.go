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

// AnnotationLine is a distinguished comment line. Its annotation meaning is
// assigned during the later annotation-validation stage.
type AnnotationLine struct {
	Raw  string
	Line int
}

func (node AnnotationLine) RawLine() string { return node.Raw }
func (node AnnotationLine) LineNumber() int { return node.Line }
func (AnnotationLine) isNode()              {}

// Variable is a structurally valid dotenv assignment. RawValue is the exact
// text after the first equals sign, while Value is its resolved content.
// HasValue distinguishes an empty resolved value from a value not yet assigned.
type Variable struct {
	Key         string
	RawValue    string
	Value       string
	HasValue    bool
	ValueSource ValueSource
	Raw         string
	Line        int
}

func (node Variable) RawLine() string { return node.Raw }
func (node Variable) LineNumber() int { return node.Line }
func (Variable) isNode()              {}
