package db

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// VerbFormInput is one cell of a verb table. The unique key is the lemma
// plus tense, person, number, and gender. Empty person, number, and gender
// are the infinitive and other forms that have none.
type VerbFormInput struct {
	LemmaID      int64
	Tense        string
	Person       string
	Number       string
	Gender       string
	FormHe       string
	Translit     string
	FormLexemeID int64
}

// VerbForm is one stored cell, in tense then person order.
type VerbForm struct {
	ID           int64
	LemmaID      int64
	Tense        string
	Person       string
	Number       string
	Gender       string
	FormHe       string
	Translit     string
	FormLexemeID int64
}

// UpsertVerbForms writes the batch in one transaction. If any lemma is missing
// from the lexicon, nothing is written.
func (s *Store) UpsertVerbForms(ctx context.Context, rows []VerbFormInput) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("db: no verb forms")
	}
	if len(rows) > maxMistakeBatch {
		return fmt.Errorf("db: at most %d verb forms", maxMistakeBatch)
	}
	clean := make([]VerbFormInput, len(rows))
	for i, row := range rows {
		row.FormHe = units.Normalize(row.FormHe)
		if row.LemmaID <= 0 {
			return MissingLemmaError{ID: row.LemmaID}
		}
		if !validTense(row.Tense) || !validPerson(row.Person) || !validNumber(row.Number) || !validGender(row.Gender) {
			return fmt.Errorf("db: verb form %d", i)
		}
		if row.FormHe == "" || utf8.RuneCountInString(row.FormHe) > maxMistakeRunes || utf8.RuneCountInString(row.Translit) > maxMistakeRunes {
			return fmt.Errorf("db: verb form %d", i)
		}
		clean[i] = row
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin verb forms: %w", err)
	}
	defer tx.Rollback()
	for _, row := range clean {
		ok, err := lexemeExists(ctx, tx, row.LemmaID)
		if err != nil {
			return fmt.Errorf("db: lemma: %w", err)
		}
		if !ok {
			return MissingLemmaError{ID: row.LemmaID}
		}
		if row.FormLexemeID > 0 {
			ok, err = lexemeExists(ctx, tx, row.FormLexemeID)
			if err != nil {
				return err
			}
			if !ok {
				return MissingLemmaError{ID: row.FormLexemeID}
			}
		}
	}
	for _, row := range clean {
		var formID any
		if row.FormLexemeID > 0 {
			formID = row.FormLexemeID
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO verb_forms (lemma_lexeme_id, tense, person, number, gender, form_he, form_translit, form_lexeme_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (lemma_lexeme_id, tense, person, number, gender) DO UPDATE SET
				form_he = excluded.form_he,
				form_translit = excluded.form_translit,
				form_lexeme_id = excluded.form_lexeme_id`,
			row.LemmaID, row.Tense, row.Person, row.Number, row.Gender, row.FormHe, row.Translit, formID); err != nil {
			return fmt.Errorf("db: verb form: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit verb forms: %w", err)
	}
	return nil
}

// VerbFormsFor returns one lemma's forms in tense order, then person order.
func (s *Store) VerbFormsFor(ctx context.Context, lemmaID int64) ([]VerbForm, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, lemma_lexeme_id, tense, person, number, gender, form_he, form_translit, COALESCE(form_lexeme_id, 0)
		FROM verb_forms
		WHERE lemma_lexeme_id = ?
		ORDER BY CASE tense
			WHEN 'present' THEN 1
			WHEN 'past' THEN 2
			WHEN 'future' THEN 3
			WHEN 'imperative' THEN 4
			WHEN 'infinitive' THEN 5
			ELSE 6 END,
			CASE person WHEN '1' THEN 1 WHEN '2' THEN 2 WHEN '3' THEN 3 ELSE 4 END,
			CASE number WHEN 'sg' THEN 1 WHEN 'pl' THEN 2 ELSE 3 END,
			CASE gender WHEN 'm' THEN 1 WHEN 'f' THEN 2 ELSE 3 END,
			id`, lemmaID)
	if err != nil {
		return nil, fmt.Errorf("db: verb forms: %w", err)
	}
	defer rows.Close()
	var out []VerbForm
	for rows.Next() {
		var row VerbForm
		if err := rows.Scan(&row.ID, &row.LemmaID, &row.Tense, &row.Person, &row.Number, &row.Gender, &row.FormHe, &row.Translit, &row.FormLexemeID); err != nil {
			return nil, fmt.Errorf("db: verb forms: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: verb forms: %w", err)
	}
	if out == nil {
		out = []VerbForm{}
	}
	return out, nil
}

// VerbFormsAll returns every stored form, lemmas with the lowest id first,
// and each lemma in tense and person order.
func (s *Store) VerbFormsAll(ctx context.Context) ([]VerbForm, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, lemma_lexeme_id, tense, person, number, gender, form_he, form_translit, COALESCE(form_lexeme_id, 0)
		FROM verb_forms
		ORDER BY lemma_lexeme_id,
			CASE tense
				WHEN 'present' THEN 1
				WHEN 'past' THEN 2
				WHEN 'future' THEN 3
				WHEN 'imperative' THEN 4
				WHEN 'infinitive' THEN 5
				ELSE 6 END,
			CASE person WHEN '1' THEN 1 WHEN '2' THEN 2 WHEN '3' THEN 3 ELSE 4 END,
			id`)
	if err != nil {
		return nil, fmt.Errorf("db: verb forms: %w", err)
	}
	defer rows.Close()
	var out []VerbForm
	for rows.Next() {
		var row VerbForm
		if err := rows.Scan(&row.ID, &row.LemmaID, &row.Tense, &row.Person, &row.Number, &row.Gender, &row.FormHe, &row.Translit, &row.FormLexemeID); err != nil {
			return nil, fmt.Errorf("db: verb forms: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: verb forms: %w", err)
	}
	if out == nil {
		out = []VerbForm{}
	}
	return out, nil
}

func validTense(s string) bool {
	switch s {
	case "present", "past", "future", "imperative", "infinitive":
		return true
	default:
		return false
	}
}

func validPerson(s string) bool {
	switch s {
	case "", "1", "2", "3":
		return true
	default:
		return false
	}
}

func validNumber(s string) bool {
	switch s {
	case "", "sg", "pl":
		return true
	default:
		return false
	}
}

func validGender(s string) bool {
	switch s {
	case "", "m", "f":
		return true
	default:
		return false
	}
}
