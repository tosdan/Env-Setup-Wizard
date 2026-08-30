# Env Setup Wizard

Env Setup Wizard turns a dotenv template into a guided, validated configuration while preserving the template's human-readable structure.

## Language

**Template**:
The `.env.example` source of truth that defines the supported variables, their order, defaults, comments, and optional annotations.
_Avoid_: Input file, schema

**Document**:
The ordered content of a Template, including variables, comments, annotation lines, blank lines, and its newline style.
_Avoid_: Configuration map, parsed map

**Variable**:
A named dotenv assignment in the Document. A Variable is configuration data, not an interaction shown to the user.
_Avoid_: Field, question

**Raw value**:
The exact dotenv text written after a Variable's assignment delimiter, including any quoting and escaping.
_Avoid_: Resolved value

**Resolved value**:
The single-line configuration content represented by a Raw value after applying dotenv semantics. An empty Resolved value still exists.
_Avoid_: Raw value, encoded value

**Value source**:
The origin of a Variable's current Resolved value: Template, Existing configuration, user answer, or fixed Template value.
_Avoid_: Default

**Annotation**:
Wizard metadata expressed as a distinguished comment line in the Template.
_Avoid_: Directive, command

**Variable type**:
The annotation-selected validation and interaction semantics of a Variable: string, integer, boolean, port, or URL.
_Avoid_: Go type, widget

**Section**:
The named wizard grouping context assigned to Variables in document order. Repeated names represent the same logical group without changing Document order.
_Avoid_: Document section, reordered block

**Question**:
A configurable interaction derived from a Variable and its annotations.
_Avoid_: Variable, field

**Existing configuration**:
An optional current `.env` used only as a source of values when the wizard is run again.
_Avoid_: Template, source of truth

**Generated configuration**:
The `.env` candidate produced from the Template and the resolved values.
_Avoid_: Template
