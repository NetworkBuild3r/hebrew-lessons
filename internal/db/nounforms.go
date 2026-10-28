package db

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// NounFormInput is one cell of a noun or adjective table. The unique key is
// the lemma plus kind, number, person, gender, and owner_number.
// INIT-023/SPEC-005
type NounFormInput struct {
	LemmaID      int64
	Kind         string
	Number       string
	Person       string
	Gender       string
	OwnerNumber  string
	FormHe       string
	Translit     string
	FormLexemeID int64
}

// NounForm is one stored cell, in reading order: singular, definite, plural,
// construct sg, construct pl, possessive by owner, then agreement.
// AudioID is the clip for this form's text when one exists.
// INIT-023/SPEC-005
type NounForm struct {
	ID           int64
	LemmaID      int64
	Kind         string
	Number       string
	Person       string
	Gender       string
	OwnerNumber  string
	FormHe       string
	Translit     string
	FormLexemeID int64
	AudioID      string
}

// UpsertNounForms writes the batch in one transaction. If any lemma is missing
// from the lexicon, nothing is written. A repeat of the same key replaces
// form_he, form_translit, and form_lexeme_id.
// INIT-023/SPEC-005
func (s *Store) UpsertNounForms(ctx context.Context, rows []NounFormInput) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("db: no noun forms")
	}
	if len(rows) > maxMistakeBatch {
		return fmt.Errorf("db: at most %d noun forms", maxMistakeBatch)
	}
	clean := make([]NounFormInput, len(rows))
	for i, row := range rows {
		row.FormHe = units.Normalize(row.FormHe)
		if row.LemmaID <= 0 {
			return MissingLemmaError{ID: row.LemmaID}
		}
		if !validNounKind(row.Kind) || !validPerson(row.Person) || !validNumber(row.Number) || !validGender(row.Gender) || !validNumber(row.OwnerNumber) {
			return fmt.Errorf("db: noun form %d", i)
		}
		if row.FormHe == "" || utf8.RuneCountInString(row.FormHe) > maxMistakeRunes || utf8.RuneCountInString(row.Translit) > maxMistakeRunes {
			return fmt.Errorf("db: noun form %d", i)
		}
		clean[i] = row
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin noun forms: %w", err)
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
			INSERT INTO noun_forms (lemma_lexeme_id, kind, number, person, gender, owner_number, form_he, form_translit, form_lexeme_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (lemma_lexeme_id, kind, number, person, gender, owner_number) DO UPDATE SET
				form_he = excluded.form_he,
				form_translit = excluded.form_translit,
				form_lexeme_id = excluded.form_lexeme_id`,
			row.LemmaID, row.Kind, row.Number, row.Person, row.Gender, row.OwnerNumber, row.FormHe, row.Translit, formID); err != nil {
			return fmt.Errorf("db: noun form: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit noun forms: %w", err)
	}
	return nil
}

// NounFormsFor returns one lemma's forms in reading order: singular, definite,
// plural, construct sg, construct pl, possessive by owner, then agreement.
// AudioID comes from the form lexeme's audio_id, or from audio_assets by form
// text (MAX(id), same as AudioIDs), or empty when neither has a clip.
// INIT-023/SPEC-005
func (s *Store) NounFormsFor(ctx context.Context, lemmaID int64) ([]NounForm, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT nf.id, nf.lemma_lexeme_id, nf.kind, nf.number, nf.person, nf.gender, nf.owner_number,
			nf.form_he, nf.form_translit, COALESCE(nf.form_lexeme_id, 0),
			COALESCE(fl.audio_id, (SELECT MAX(a.id) FROM audio_assets AS a WHERE a.text_nfc = nf.form_he), '')
		FROM noun_forms AS nf
		LEFT JOIN lexemes AS fl ON fl.id = nf.form_lexeme_id
		WHERE nf.lemma_lexeme_id = ?
		ORDER BY CASE nf.kind
			WHEN 'singular' THEN 1
			WHEN 'definite' THEN 2
			WHEN 'plural' THEN 3
			WHEN 'construct' THEN 4
			WHEN 'possessive' THEN 5
			WHEN 'agreement' THEN 6
			ELSE 7 END,
			CASE
				WHEN nf.kind = 'possessive' THEN CASE nf.person WHEN '1' THEN 1 WHEN '2' THEN 2 WHEN '3' THEN 3 ELSE 4 END
				ELSE 0
			END,
			CASE
				WHEN nf.kind = 'possessive' THEN CASE nf.gender WHEN 'm' THEN 1 WHEN 'f' THEN 2 ELSE 3 END
				WHEN nf.kind IN ('construct', 'agreement') THEN CASE nf.number WHEN 'sg' THEN 1 WHEN 'pl' THEN 2 ELSE 3 END
				ELSE 0
			END,
			CASE
				WHEN nf.kind = 'possessive' THEN CASE nf.owner_number WHEN 'sg' THEN 1 WHEN 'pl' THEN 2 ELSE 3 END
				WHEN nf.kind = 'agreement' THEN CASE nf.gender WHEN 'm' THEN 1 WHEN 'f' THEN 2 ELSE 3 END
				ELSE 0
			END,
			CASE
				WHEN nf.kind = 'possessive' THEN CASE nf.number WHEN 'sg' THEN 1 WHEN 'pl' THEN 2 ELSE 3 END
				ELSE 0
			END,
			nf.id`, lemmaID)
	if err != nil {
		return nil, fmt.Errorf("db: noun forms: %w", err)
	}
	defer rows.Close()
	var out []NounForm
	for rows.Next() {
		var row NounForm
		if err := rows.Scan(
			&row.ID, &row.LemmaID, &row.Kind, &row.Number, &row.Person, &row.Gender, &row.OwnerNumber,
			&row.FormHe, &row.Translit, &row.FormLexemeID, &row.AudioID,
		); err != nil {
			return nil, fmt.Errorf("db: noun forms: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: noun forms: %w", err)
	}
	if out == nil {
		out = []NounForm{}
	}
	return out, nil
}

func validNounKind(s string) bool {
	switch s {
	case "singular", "plural", "definite", "construct", "possessive", "agreement":
		return true
	default:
		return false
	}
}
