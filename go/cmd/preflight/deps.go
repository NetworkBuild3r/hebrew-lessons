package main

// Dependency notes for the rebase that wires the real packages.
// Do not copy go/internal/textkey or go/internal/lessondata into this branch.
//
// PR #37 (branch cursor/learner-path-readonly-c123) exports
//
//	func textkey.ExactKey(s string) string
//
// PR #25 (branch cursor/go-2-lessondata-parser-97f7) exports
//
//	func lessondata.Parse(src, file string) (lessondata.Document, error)
//	func (d lessondata.Document) Write() string
//
// When both are on main, replace productionDeps with:
//
//	func productionDeps() Deps {
//		return Deps{
//			ExactKey: func(s string) (string, error) {
//				return textkey.ExactKey(s), nil
//			},
//			RoundTrip: func(path string, src []byte) ([]byte, error) {
//				doc, err := lessondata.Parse(string(src), path)
//				if err != nil {
//					return nil, err
//				}
//				// Indented writer. A stored file that is not already those
//				// bytes does not round-trip, which is the cutover finding.
//				return []byte(doc.Write()), nil
//			},
//		}
//	}

const (
	textkeyDep    = "go/internal/textkey (PR #37, branch cursor/learner-path-readonly-c123)"
	lessondataDep = "go/internal/lessondata (PR #25, branch cursor/go-2-lessondata-parser-97f7)"
)

// productionDeps leaves ExactKey and RoundTrip nil until those pulls merge.
// A nil dep is a pending check, and a pending check fails the process:
// this command has no warn level, and a check that did not run must not
// exit 0.
func productionDeps() Deps {
	return Deps{}
}
