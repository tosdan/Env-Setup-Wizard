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

**Annotation**:
Wizard metadata expressed as a distinguished comment line in the Template.
_Avoid_: Directive, command

**Question**:
A configurable interaction derived from a Variable and its annotations.
_Avoid_: Variable, field

**Existing configuration**:
An optional current `.env` used only as a source of values when the wizard is run again.
_Avoid_: Template, source of truth

**Generated configuration**:
The `.env` candidate produced from the Template and the resolved values.
_Avoid_: Template
