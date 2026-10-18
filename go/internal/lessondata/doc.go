// Package lessondata parses and writes lesson worksheet and catalog files.
//
// Those files are data, not programs: one or more
//
//	window.NAME = <literal>;
//
// statements, plus the one Object.assign merge shape the word-audio catalogs
// use. This is a port of server/lesson-data.js. It never executes the file.
// Grammar, rejected forms, and SyntaxError text match that parser, including
// the tokenizer's habit of reporting string errors at the opening quote.
//
// Writers match the three byte-for-byte forms in ADR-0016 §3:
//
//	window.NAME = <JSON.stringify(value, null, 4)>;\n
//	window.NAME = Object.assign({}, window.NAME || {}, <JSON.stringify(value, null, 4)>);\n
//	window.NAME = <JSON.stringify(value).replace(/</g, "\\u003c")>;\n
package lessondata
