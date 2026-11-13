package views

// The ways of the Jews: the handful of things a reader needs to follow Isaiah.
//
// This is reference, not a course (ADR-0035 D5). Every entry is a fixed fact
// about practice, not a claim about a verse. A verse that uses one of these
// links to it from the Isaiah page. The copy is deliberately plain and does not
// tell the learner what to believe. INIT-040.
//
// The entries are chosen because Isaiah leans on them: the Temple and its
// service, the festivals that remember the exodus and the harvest, the fasts
// that remember the destruction, and the words for justice, remnant, and
// holiness that the prophet repeats.

// Custom is one practice or idea, with what it means and why it matters for
// reading the prophets.
type Custom struct {
	// Key is the slug used in a URL, such as "shabbat".
	Key string
	// Group is the group it belongs to: "Time", "The Temple", "The Prophets", or "Words".
	Group string
	// Hebrew is the name in Hebrew, pointed.
	Hebrew string
	// Name is the name in Latin letters.
	Name string
	// What is one sentence saying what it is.
	What string
	// Why is one sentence saying why it helps read Isaiah.
	Why string
	// Isaiah is a pointer to where the prophet leans on it, when there is one.
	Isaiah string
}

// CustomGroups is the order the groups appear in.
var CustomGroups = []string{"Time", "The Temple", "The Prophets", "Words"}

// Customs is the reference, in teaching order.
var Customs = []Custom{
	// Time: the year a prophet lived in.
	{
		Key: "shabbat", Group: "Time",
		Hebrew: "שַׁבָּת", Name: "Shabbat",
		What: "From Friday sunset to Saturday night, work stops. The week is built around it, not the other way round.",
		Why:  "Isaiah calls it a delight and ties it to inheritance, which is the promise language of the whole book.",
	},
	{
		Key: "pesach", Group: "Time",
		Hebrew: "פֶּסַח", Name: "Pesaḥ",
		What: "The spring festival of the exodus from Egypt, kept with a meal and the retelling of the story.",
		Why:  "The prophets treat the exodus as the proof that God saves, and Isaiah promises a second one.",
	},
	{
		Key: "shavuot", Group: "Time",
		Hebrew: "שָׁבוּעוֹת", Name: "Shavuot",
		What: "Seven weeks after Pesaḥ, the wheat harvest and the giving of the Torah.",
		Why:  "It is the harvest frame behind the vineyard and threshing-floor pictures in Isaiah 5 and 17.",
	},
	{
		Key: "sukkot", Group: "Time",
		Hebrew: "סוּכּוֹת", Name: "Sukkot",
		What: "The autumn festival of booths, when meals are eaten in a temporary shelter for a week.",
		Why:  "Isaiah's picture of every nation coming up to Jerusalem is read against this festival.",
	},
	{
		Key: "yom-kippur", Group: "Time",
		Hebrew: "יוֹם כִּפּוּר", Name: "Yom Kippur",
		What: "The day of atonement, a fast from sunset to sunset, once a year.",
		Why:  "Isaiah 58 argues about what a fast is for, and this is the fast he has in view.",
	},
	{
		Key: "tisha-bav", Group: "Time",
		Hebrew: "תִּשְׁעָה בְּאָב", Name: "Tish'a b'Av",
		What: "A summer fast for the destruction of both Temples.",
		Why:  "The destruction Isaiah warns about is the one this day remembers.",
	},

	// The Temple: where the prophets stand.
	{
		Key: "beit-hamikdash", Group: "The Temple",
		Hebrew: "בֵּית הַמִּקְדָּשׁ", Name: "The Temple",
		What: "The house in Jerusalem where sacrifice was offered and where God's name was said to dwell.",
		Why:  "Isaiah's call vision happens inside it, and its fate is the question the book keeps returning to.",
	},
	{
		Key: "korban", Group: "The Temple",
		Hebrew: "קָרְבָּן", Name: "Korban",
		What: "An offering brought near. The Hebrew word comes from the root for drawing close.",
		Why:  "Isaiah 1 rejects offerings brought without justice, which only lands if you know what an offering was.",
	},
	{
		Key: "kohen", Group: "The Temple",
		Hebrew: "כֹּהֵן", Name: "Kohen",
		What: "A priest. The role passes down a family line, and priests teach and inspect as well as sacrifice.",
		Why:  "Isaiah's own words about the priests and their failure are about these people, not a general idea of clergy.",
	},
	{
		Key: "navi", Group: "The Temple",
		Hebrew: "נָבִיא", Name: "Navi",
		What: "A prophet. Not mainly a fortune-teller: someone who speaks in God's name to the people of his own day.",
		Why:  "It changes how to read the book. The first audience is the people listening, not the reader now.",
	},

	// The Prophets: how the book sits in the library.
	{
		Key: "neviim", Group: "The Prophets",
		Hebrew: "נְבִיאִים", Name: "Nevi'im",
		What: "The second part of the Hebrew Bible: the former prophets, the latter prophets, and the twelve.",
		Why:  "Isaiah is one book in that set, which is why its order and its neighbours matter.",
	},
	{
		Key: "haftarah", Group: "The Prophets",
		Hebrew: "הַפְטָרָה", Name: "Haftarah",
		What: "The reading from the prophets that follows the Torah portion on Shabbat and festivals.",
		Why:  "It is how most Jews have heard Isaiah: in short readings tied to the week, not read cover to cover.",
	},
	{
		Key: "teshuvah", Group: "The Prophets",
		Hebrew: "תְּשׁוּבָה", Name: "Teshuvah",
		What: "Turning, or returning. The prophets' word for coming back to the right way, not only feeling sorry.",
		Why:  "Isaiah's warnings are almost always paired with a way back. Reading only the warning misses half of it.",
	},
	{
		Key: "tzedek", Group: "The Prophets",
		Hebrew: "צֶדֶק", Name: "Tzedek",
		What: "Justice, in the sense of right order between people, not only a court verdict.",
		Why:  "It is the single most repeated demand in the book, and the measure every warning is set against.",
	},
	{
		Key: "mishpat", Group: "The Prophets",
		Hebrew: "מִשְׁפָּט", Name: "Mishpat",
		What: "Judgement, or the practice of right judgement. It sits beside tzedek so often the pair reads as one idea.",
		Why:  "Knowing the pair is the quickest way to hear what Isaiah means by a just society.",
	},

	// Words: the vocabulary the prophet keeps using.
	{
		Key: "shear-yashuv", Group: "Words",
		Hebrew: "שְׁאָר יָשׁוּב", Name: "She'ar yashuv",
		What: "A remnant will return. The name of Isaiah's son, and the hope inside the warnings.",
		Why:  "It is the book's own answer to the question of whether anything survives.",
	},
	{
		Key: "kadosh", Group: "Words",
		Hebrew: "קָדוֹשׁ", Name: "Kadosh",
		What: "Holy: set apart, not simply good.",
		Why:  "The call vision repeats it three times, and the whole book takes its shape from that scene.",
	},
	{
		Key: "goel", Group: "Words",
		Hebrew: "גּוֹאֵל", Name: "Go'el",
		What: "A redeemer: the near relative who buys back what a family has lost.",
		Why:  "It is a family and property law word that the prophets borrow for the whole nation.",
	},
	{
		Key: "or", Group: "Words",
		Hebrew: "אוֹר", Name: "Or",
		What: "Light.",
		Why:  "Light against darkness runs through the book, and the people walking in darkness is its most quoted line.",
	},
	{
		Key: "derekh", Group: "Words",
		Hebrew: "דֶּרֶךְ", Name: "Derekh",
		What: "A road, or a way of living. The same word for both.",
		Why:  "The highway in the wilderness and the way of the wicked are the same word doing two jobs.",
	},
}

// CustomsByGroup groups the reference in the order the groups appear.
func CustomsByGroup() []CustomGroupView {
	var out []CustomGroupView
	for _, group := range CustomGroups {
		g := CustomGroupView{Name: group}
		for _, c := range Customs {
			if c.Group == group {
				g.Items = append(g.Items, c)
			}
		}
		if len(g.Items) > 0 {
			out = append(out, g)
		}
	}
	return out
}

// CustomGroupView is one group of the reference, ready to render.
type CustomGroupView struct {
	Name  string
	Items []Custom
}
