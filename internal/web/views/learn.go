package views

import "github.com/NetworkBuild3r/hebrew-lessons/internal/richtext"

// LearnGroup is one published explainer on the index, in path order.
type LearnGroup struct {
	Slug    string
	Title   string
	Summary string
	Example string
}

// LearnIndexView is the Learn index.
type LearnIndexView struct {
	Page   Page
	Groups []LearnGroup
}

// LearnUsed is a lesson that teaches the pattern.
type LearnUsed struct {
	Href  string
	Title string
}

// LearnOften is one published shared mistake for an explainer.
type LearnOften struct {
	Wrong   string
	Right   string
	Explain string
}

// LearnPageView is one explainer.
type LearnPageView struct {
	Page  Page
	Title string
	Nodes []richtext.Node
	Used  []LearnUsed
	Often []LearnOften
}

// LearnSheetView is the explainer fragment the lesson sheet loads.
type LearnSheetView struct {
	Title string
	Nodes []richtext.Node
}
