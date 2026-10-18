package path

// Build is the dev course. due is the queue count. When ack is true the
// not-yet unit has already been warned, so its nodes are data-gate="open".
// A review stop is inserted only when due meets the config threshold.
func Build(cfg Config, due int, ack bool) Snapshot {
	gate := "warn"
	if ack {
		gate = "open"
	}
	cafeCheck := "/worksheets/cafe-checkpoint/"
	outCheck := "/worksheets/out-and-about-checkpoint/"
	units := []Unit{
		{
			ID: "unit-1", Order: 1, Title: "Getting around", TitleHe: "מִסְתּוֹבְבִים",
			NewLearned: 6, NewTotal: 6, LessonCount: 3,
			Nodes: []Node{
				lesson("unit-1", "hello", "Hello", "שָׁלוֹם", "done", "", 1, 2, 4, 4, false),
				lesson("unit-1", "what-i-have", "What I have", "יֵשׁ לִי", "done", "", 2, 2, 8, 1, true),
				lesson("unit-1", "where-is-it", "Where is it", "אֵיפֹה זֶה", "done", "", 3, 2, 10, 3, false),
			},
		},
		{
			ID: "unit-2", Order: 2, Title: "Out and about", TitleHe: "בַּחוּץ",
			Concept: "Infinitive", ConceptHe: "שֵׁם הַפֹּעַל",
			NewLearned: 5, NewTotal: 8, Current: true, LessonIndex: 3, LessonCount: 6,
			GuideHref: "/guides/",
			Nodes: []Node{
				lesson("unit-2", "morning", "Morning", "בֹּקֶר", "done", "", 1, 2, 6, 4, false),
				lesson("unit-2", "the-street", "The street", "הָרְחוֹב", "done", "", 2, 3, 10, 3, false),
				lesson("unit-2", "out-the-door", "Out the door", "יוֹצְאִים", "current", "", 3, 3, 12, 3, false),
				lesson("unit-2", "at-the-shop", "At the shop", "בַּחֲנוּת", "not-yet", "open", 4, 2, 12, 0, false),
				lesson("unit-2", "asking", "Asking", "לִשְׁאוֹל", "not-yet", "open", 5, 1, 12, 0, false),
				checkpoint("unit-2", "out-and-about-checkpoint", outCheck, "open", 20, nil),
			},
		},
		{
			ID: "unit-3", Order: 3, Title: "At the café", TitleHe: "בְּבֵית קָפֶה",
			NewLearned: 0, NewTotal: 8, NotYet: true, LessonCount: 3,
			Nodes: []Node{
				lesson("unit-3", "coffee", "Coffee", "קָפֶה", "not-yet", gate, 1, 2, 0, 0, false),
				lesson("unit-3", "the-bill", "The bill", "הַחֶשְׁבּוֹן", "not-yet", gate, 2, 2, 0, 0, false),
				checkpoint("unit-3", "cafe-checkpoint", cafeCheck, gate, 20, nil),
			},
		},
		{
			ID: "unit-tutor", Order: 4, Title: "Class with Aaron", TitleHe: "שִׁעוּר עִם אַהֲרֹן",
			NewLearned: 0, NewTotal: 4, LessonCount: 1,
			Nodes: []Node{
				{
					Slug: "class-with-aaron", Title: "Class with Aaron", TitleHe: "שִׁעוּר עִם אַהֲרֹן",
					Kind: "tutor", State: "tutor", Gate: "open", UnitID: "unit-tutor",
					ClassDate: "Tue", ClassDateHe: "יום ג׳", ClassDateISO: "2026-10-20",
					Href: "/worksheets/class-with-aaron/", LessonNumber: 1,
				},
			},
		},
	}
	for ui := range units {
		for ni := range units[ui].Nodes {
			if units[ui].Nodes[ni].Kind == "checkpoint" || units[ui].Nodes[ni].Kind == "lesson" {
				if units[ui].ID == "unit-3" {
					units[ui].Nodes[ni].CheckpointHref = cafeCheck
				}
				if units[ui].ID == "unit-2" {
					units[ui].Nodes[ni].CheckpointHref = outCheck
				}
			}
		}
	}
	if due >= cfg.ReviewStopDueCount {
		u := &units[1]
		stop := Node{
			Title: "Review", TitleHe: "חזרה", Kind: "review", State: "review",
			UnitID: u.ID, Due: due, Href: "/review/",
		}
		nodes := make([]Node, 0, len(u.Nodes)+1)
		for _, n := range u.Nodes {
			nodes = append(nodes, n)
			if n.State == "current" {
				nodes = append(nodes, stop)
			}
		}
		u.Nodes = nodes
	}
	snap := Snapshot{
		PlanID: DemoPlanID, UnitCount: 12, Units: units,
		Due: due, DueKnown: true,
		DueWords: []DueWord{
			{He: "שָׁלוֹם", En: "hello", Strength: 2, Speak: "שָׁלוֹם"},
			{He: "בְּבַקָּשָׁה", En: "please", Strength: 1, Speak: "בְּבַקָּשָׁה"},
			{He: "תּוֹדָה", En: "thank you", Strength: 3, Speak: "תּוֹדָה"},
		},
		Tutor: &TutorCard{
			UnitID: "unit-tutor", DateLabel: "Tue", DateHe: "יום ג׳", DateISO: "2026-10-20",
			SentenceEn: "Tuesday's class practices the infinitive.",
			SentenceHe: "השיעור ביום שלישי מתרגל את שם הפועל.",
			Phrase:     "בֹּקֶר טוֹב", PhraseEn: "good morning",
			Href: "/lessons/#unit-unit-tutor",
		},
	}
	return Present(cfg, snap)
}

// Demo is the course the dev server renders: 9 words due, gate not yet acked.
func Demo() Snapshot {
	return Build(DefaultConfig(), 9, false)
}

func lesson(unit, slug, title, titleHe, state, gate string, number, newCount, known, strength int, fading bool) Node {
	return Node{
		Slug: slug, Title: title, TitleHe: titleHe, Kind: "lesson", State: state, Gate: gate,
		UnitID: unit, LessonNumber: number, NewCount: newCount, KnownCount: known,
		Strength: strength, Fading: fading, Href: "/worksheets/" + slug + "/",
	}
}

func checkpoint(unit, slug, href, gate string, total int, score *int) Node {
	return Node{
		Slug: slug, Title: "Checkpoint", TitleHe: "מִבְחָן", Kind: "checkpoint", State: "checkpoint",
		Gate: gate, UnitID: unit, Href: href, CheckpointHref: href, Total: total, Score: score,
	}
}
