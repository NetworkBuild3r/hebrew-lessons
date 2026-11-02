package placement

import "testing"

func TestDeckFileOrderAndFields(t *testing.T) {
	rows, err := Deck()
	if err != nil {
		t.Fatal(err)
	}
	cat, err := catalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(cat) {
		t.Fatalf("deck %d catalog %d", len(rows), len(cat))
	}
	for i, row := range rows {
		it := cat[i]
		if row.ID != it.ID || row.Band != it.Band || row.Verified != it.Verified || row.GlossEN != it.GlossEN {
			t.Fatalf("row %d = %+v, catalog %+v", i, row, it)
		}
	}
	if len(rows) < 2 || rows[0].ID != cat[0].ID {
		t.Fatalf("file order broken: first %s", rows[0].ID)
	}
}
