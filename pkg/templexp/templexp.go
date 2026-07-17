package templexp

import (
	"errors"
	"fmt"
	"strings"
)

// LookupFunc resolves a variable by name. The boolean reports whether the
// variable is set, independently of whether its value is empty.
type LookupFunc func(name string) (value string, found bool)

// SyntaxError reports an invalid interpolation expression.
type SyntaxError struct {
	Offset  int
	Message string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("templexp: syntax error at byte %d: %s", e.Offset, e.Message)
}

// RequiredError reports a variable rejected by ? or :?.
type RequiredError struct {
	Name    string
	Message string
	Offset  int
}

func (e *RequiredError) Error() string {
	return fmt.Sprintf("templexp: %s: %s", e.Name, e.Message)
}

type operator uint8

const (
	opValue operator = iota
	opDefaultIfUnset
	opDefaultIfEmpty
	opAlternateIfSet
	opAlternateIfNonEmpty
	opRequiredIfUnset
	opRequiredIfEmpty
)

type template struct {
	parts []part
}

type part struct {
	literal   string
	expansion *expansion
}

type expansion struct {
	name   string
	op     operator
	word   template
	offset int
}

type parser struct {
	text   string
	offset int
	depth  int
}

const maxNestingDepth = 100

func (p *parser) parse(stopAtBrace bool, openingOffset int) (template, error) {
	result := template{}
	var literal strings.Builder

	flushLiteral := func() {
		if literal.Len() == 0 {
			return
		}
		result.parts = append(result.parts, part{literal: literal.String()})
		literal.Reset()
	}

	for p.offset < len(p.text) {
		if stopAtBrace && p.text[p.offset] == '}' {
			p.offset++
			flushLiteral()

			return result, nil
		}

		if p.text[p.offset] != '$' || p.offset+1 >= len(p.text) {
			literal.WriteByte(p.text[p.offset])
			p.offset++
			continue
		}

		switch p.text[p.offset+1] {
		case '$':
			literal.WriteByte('$')
			p.offset += 2
		case '{':
			flushLiteral()
			expr, err := p.parseExpansion()
			if err != nil {
				return template{}, err
			}
			result.parts = append(result.parts, part{expansion: &expr})
		default:
			literal.WriteByte('$')
			p.offset++
		}
	}

	if stopAtBrace {
		return template{}, syntaxError(openingOffset, "unclosed interpolation")
	}
	flushLiteral()

	return result, nil
}

func (p *parser) parseExpansion() (expansion, error) {
	openingOffset := p.offset
	if p.depth >= maxNestingDepth {
		return expansion{}, syntaxError(openingOffset, "maximum interpolation nesting depth exceeded")
	}
	p.depth++
	defer func() { p.depth-- }()

	p.offset += 2
	nameStart := p.offset
	if p.offset >= len(p.text) || !isNameStart(p.text[p.offset]) {
		return expansion{}, syntaxError(p.offset, "expected variable name")
	}
	p.offset++
	for p.offset < len(p.text) && isNameChar(p.text[p.offset]) {
		p.offset++
	}

	expr := expansion{name: p.text[nameStart:p.offset], offset: openingOffset}
	if p.offset >= len(p.text) {
		return expansion{}, syntaxError(openingOffset, "unclosed interpolation")
	}
	if p.text[p.offset] == '}' {
		p.offset++

		return expr, nil
	}

	op, width, ok := parseOperator(p.text[p.offset:])
	if !ok {
		return expansion{}, syntaxError(p.offset, "unsupported or invalid operator")
	}
	expr.op = op
	p.offset += width

	word, err := p.parse(true, openingOffset)
	if err != nil {
		return expansion{}, err
	}
	expr.word = word

	return expr, nil
}

func parseOperator(text string) (operator, int, bool) {
	if len(text) >= 2 && text[0] == ':' {
		switch text[1] {
		case '-':
			return opDefaultIfEmpty, 2, true
		case '+':
			return opAlternateIfNonEmpty, 2, true
		case '?':
			return opRequiredIfEmpty, 2, true
		}
	}
	if text == "" {
		return opValue, 0, false
	}

	switch text[0] {
	case '-':
		return opDefaultIfUnset, 1, true
	case '+':
		return opAlternateIfSet, 1, true
	case '?':
		return opRequiredIfUnset, 1, true
	default:
		return opValue, 0, false
	}
}

func isNameStart(ch byte) bool {
	return ch == '_' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z'
}

func isNameChar(ch byte) bool {
	return isNameStart(ch) || ch >= '0' && ch <= '9'
}

func syntaxError(offset int, message string) error {
	return &SyntaxError{Offset: offset, Message: message}
}

type resolvedValue struct {
	value string
	found bool
}

type evaluator struct {
	lookup LookupFunc
	cache  map[string]resolvedValue
}

func (e *evaluator) expand(tmpl template) (string, error) {
	var result strings.Builder
	for _, item := range tmpl.parts {
		if item.expansion == nil {
			result.WriteString(item.literal)
			continue
		}

		value, err := e.expandVariable(*item.expansion)
		if err != nil {
			return "", err
		}
		result.WriteString(value)
	}

	return result.String(), nil
}

func (e *evaluator) expandVariable(expr expansion) (string, error) {
	resolved := e.resolve(expr.name)
	switch expr.op {
	case opValue:
		return resolved.value, nil
	case opDefaultIfUnset:
		return e.expandWordUnless(expr, resolved.found)
	case opDefaultIfEmpty:
		return e.expandWordUnless(expr, resolved.found && resolved.value != "")
	case opAlternateIfSet:
		return e.expandWordWhen(expr, resolved.found)
	case opAlternateIfNonEmpty:
		return e.expandWordWhen(expr, resolved.found && resolved.value != "")
	case opRequiredIfUnset:
		return e.require(expr, resolved, resolved.found, "required variable is unset")
	case opRequiredIfEmpty:
		return e.require(expr, resolved, resolved.found && resolved.value != "", "required variable is unset or empty")
	}

	return "", syntaxError(expr.offset, "unknown operator")
}

func (e *evaluator) expandWordUnless(expr expansion, condition bool) (string, error) {
	if condition {
		return e.resolve(expr.name).value, nil
	}

	return e.expand(expr.word)
}

func (e *evaluator) expandWordWhen(expr expansion, condition bool) (string, error) {
	if !condition {
		return "", nil
	}

	return e.expand(expr.word)
}

func (e *evaluator) require(expr expansion, resolved resolvedValue, valid bool, defaultMessage string) (string, error) {
	if valid {
		return resolved.value, nil
	}

	message, err := e.expand(expr.word)
	if err != nil {
		return "", err
	}
	if message == "" {
		message = defaultMessage
	}

	return "", &RequiredError{Name: expr.name, Message: message, Offset: expr.offset}
}

func (e *evaluator) resolve(name string) resolvedValue {
	if value, ok := e.cache[name]; ok {
		return value
	}

	value, found := e.lookup(name)
	resolved := resolvedValue{value: value, found: found}
	e.cache[name] = resolved

	return resolved
}

// Expand interpolates text using a read-only Docker Compose-style subset:
//
//   - ${VAR} substitutes a value, or an empty string when VAR is unset.
//   - ${VAR:-word} and ${VAR-word} provide default values.
//   - ${VAR:+word} and ${VAR+word} provide alternate values.
//   - ${VAR:?word} and ${VAR?word} require values.
//   - $$ emits a literal dollar sign.
//
// A colon makes an operator treat an empty value like an unset variable. Word
// may contain nested interpolations. Assignment operators are not supported.
func Expand(text string, lookup LookupFunc) (string, error) {
	if lookup == nil {
		return "", errors.New("templexp: nil lookup function")
	}

	p := parser{text: text}
	tmpl, err := p.parse(false, 0)
	if err != nil {
		return "", err
	}

	e := evaluator{lookup: lookup, cache: make(map[string]resolvedValue)}

	return e.expand(tmpl)
}
