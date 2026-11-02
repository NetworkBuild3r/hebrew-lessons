package letters

import "testing"

func TestLookAlikeOrderAndContents(t *testing.T) {
	want := []string{
		"ב/כ", "ד/ר", "ה/ח", "ח/ת", "ו/ז", "ו/ן", "ט/מ", "ם/ס", "ג/נ", "ע/צ",
		"כ/פ", "ך/ף", "ם/מ", "ן/נ", "ך/כ", "ף/פ", "ץ/צ", "ג/פ", "ו/י",
	}
	if len(LookAlike) != len(want) {
		t.Fatalf("LookAlike len = %d, want %d", len(LookAlike), len(want))
	}
	for i, key := range want {
		if LookAlike[i] != key {
			t.Fatalf("LookAlike[%d] = %q, want %q", i, LookAlike[i], key)
		}
	}
}

func TestSoundAlikeOrderAndContents(t *testing.T) {
	want := []string{"א/ע", "ט/ת", "כ/ק", "ח/כ", "ב/ו", "ס/ש"}
	if len(SoundAlike) != len(want) {
		t.Fatalf("SoundAlike len = %d, want %d", len(SoundAlike), len(want))
	}
	for i, key := range want {
		if SoundAlike[i] != key {
			t.Fatalf("SoundAlike[%d] = %q, want %q", i, SoundAlike[i], key)
		}
	}
}

func TestConfusable(t *testing.T) {
	for _, key := range []string{"ב/כ", "כ/ב", "א/ע", "ע/א", "ו/י", "ח/כ", "ם/ס"} {
		if !Confusable(key) {
			t.Fatalf("Confusable(%q) = false, want true", key)
		}
	}
	for _, key := range []string{"", "ב", "ב/", "/כ", "ו/מ", "מ/ו", "ב/כ/ד", "a/b"} {
		if Confusable(key) {
			t.Fatalf("Confusable(%q) = true, want false", key)
		}
	}
}

func TestBarePair(t *testing.T) {
	got, ok := BarePair("בּ", "כּ")
	if !ok || got != "ב/כ" {
		t.Fatalf("BarePair(בּ, כּ) = %q, %v; want ב/כ, true", got, ok)
	}
	if _, ok := BarePair("שׁ", "שׂ"); ok {
		t.Fatal("BarePair(שׁ, שׂ) should drop the collapsed pair")
	}
}
