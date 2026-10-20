package views

import (
	"strconv"

	"github.com/a-h/templ"
)

// ButtonKind is the Figma Button variant. Primary is the one main action on a
// screen (--action-bg). Quiet is every secondary action: outlined, no fill.
type ButtonKind string

const (
	ButtonPrimary ButtonKind = "primary"
	ButtonQuiet   ButtonKind = "quiet"
)

// ButtonKinds lists every kind, in Figma order.
var ButtonKinds = []ButtonKind{ButtonPrimary, ButtonQuiet}

// normal returns k, or Quiet for an unknown kind. An unknown kind must never
// paint a second filled action.
func (k ButtonKind) normal() ButtonKind {
	if k == ButtonPrimary {
		return ButtonPrimary
	}
	return ButtonQuiet
}

// AnswerState is the Figma AnswerOption variant. There are exactly four.
type AnswerState string

const (
	AnswerDefault  AnswerState = "default"
	AnswerSelected AnswerState = "selected"
	AnswerCorrect  AnswerState = "correct"
	AnswerWrong    AnswerState = "wrong"
)

// AnswerStates lists the four Figma states, in Figma order.
var AnswerStates = []AnswerState{AnswerDefault, AnswerSelected, AnswerCorrect, AnswerWrong}

// normal returns s, or Default for an unknown state.
func (s AnswerState) normal() AnswerState {
	for _, known := range AnswerStates {
		if s == known {
			return s
		}
	}
	return AnswerDefault
}

// buttonAttrs copies attrs and defaults type to "button", so a component
// inside a form does not submit it by accident.
func buttonAttrs(attrs templ.Attributes) templ.Attributes {
	out := templ.Attributes{}
	for k, v := range attrs {
		out[k] = v
	}
	if _, ok := out["type"]; !ok {
		out["type"] = "button"
	}
	return out
}

// progressRange clamps a progress value into [0, max]. A max below 1 becomes 1.
func progressRange(value, max int) (string, string) {
	if max < 1 {
		max = 1
	}
	if value < 0 {
		value = 0
	}
	if value > max {
		value = max
	}
	return strconv.Itoa(value), strconv.Itoa(max)
}
