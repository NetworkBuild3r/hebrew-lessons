package db

import "strings"

// TanakhBook is one book of the shipped public-domain corpus.
// Verses is the count the importer requires for that slug.
type TanakhBook struct {
	Slug   string
	Title  string
	Verses int
}

// tanakhCanon is the 39 books in canonical order, with the verse counts
// of the Sefaria fetch pinned to Tanach with Nikkud and JPS 1917.
var tanakhCanon = []TanakhBook{
	{Slug: "genesis", Title: "Genesis", Verses: 1533},
	{Slug: "exodus", Title: "Exodus", Verses: 1210},
	{Slug: "leviticus", Title: "Leviticus", Verses: 859},
	{Slug: "numbers", Title: "Numbers", Verses: 1288},
	{Slug: "deuteronomy", Title: "Deuteronomy", Verses: 956},
	{Slug: "joshua", Title: "Joshua", Verses: 658},
	{Slug: "judges", Title: "Judges", Verses: 618},
	{Slug: "i-samuel", Title: "I Samuel", Verses: 811},
	{Slug: "ii-samuel", Title: "II Samuel", Verses: 695},
	{Slug: "i-kings", Title: "I Kings", Verses: 817},
	{Slug: "ii-kings", Title: "II Kings", Verses: 719},
	{Slug: "isaiah", Title: "Isaiah", Verses: 1291},
	{Slug: "jeremiah", Title: "Jeremiah", Verses: 1364},
	{Slug: "ezekiel", Title: "Ezekiel", Verses: 1273},
	{Slug: "hosea", Title: "Hosea", Verses: 197},
	{Slug: "joel", Title: "Joel", Verses: 73},
	{Slug: "amos", Title: "Amos", Verses: 146},
	{Slug: "obadiah", Title: "Obadiah", Verses: 21},
	{Slug: "jonah", Title: "Jonah", Verses: 48},
	{Slug: "micah", Title: "Micah", Verses: 105},
	{Slug: "nahum", Title: "Nahum", Verses: 47},
	{Slug: "habakkuk", Title: "Habakkuk", Verses: 56},
	{Slug: "zephaniah", Title: "Zephaniah", Verses: 53},
	{Slug: "haggai", Title: "Haggai", Verses: 38},
	{Slug: "zechariah", Title: "Zechariah", Verses: 211},
	{Slug: "malachi", Title: "Malachi", Verses: 55},
	{Slug: "psalms", Title: "Psalms", Verses: 2527},
	{Slug: "proverbs", Title: "Proverbs", Verses: 915},
	{Slug: "job", Title: "Job", Verses: 1070},
	{Slug: "song-of-songs", Title: "Song of Songs", Verses: 117},
	{Slug: "ruth", Title: "Ruth", Verses: 85},
	{Slug: "lamentations", Title: "Lamentations", Verses: 154},
	{Slug: "ecclesiastes", Title: "Ecclesiastes", Verses: 222},
	{Slug: "esther", Title: "Esther", Verses: 167},
	{Slug: "daniel", Title: "Daniel", Verses: 357},
	{Slug: "ezra", Title: "Ezra", Verses: 280},
	{Slug: "nehemiah", Title: "Nehemiah", Verses: 405},
	{Slug: "i-chronicles", Title: "I Chronicles", Verses: 943},
	{Slug: "ii-chronicles", Title: "II Chronicles", Verses: 822},
}

// Extra names for CanonicalBook. Titles and slugs are registered from tanakhCanon.
var tanakhExtraAliases = []struct{ alias, slug string }{
	{"gen", "genesis"},
	{"gn", "genesis"},
	{"bereshit", "genesis"},
	{"בראשית", "genesis"},
	{"ex", "exodus"},
	{"exod", "exodus"},
	{"shemot", "exodus"},
	{"שמות", "exodus"},
	{"lev", "leviticus"},
	{"vayikra", "leviticus"},
	{"ויקרא", "leviticus"},
	{"num", "numbers"},
	{"bamidbar", "numbers"},
	{"במדבר", "numbers"},
	{"deut", "deuteronomy"},
	{"devarim", "deuteronomy"},
	{"דברים", "deuteronomy"},
	{"josh", "joshua"},
	{"yehoshua", "joshua"},
	{"יהושע", "joshua"},
	{"judg", "judges"},
	{"shoftim", "judges"},
	{"שופטים", "judges"},
	{"1 samuel", "i-samuel"},
	{"1 sam", "i-samuel"},
	{"1sam", "i-samuel"},
	{"1samuel", "i-samuel"},
	{"first samuel", "i-samuel"},
	{"שמואל א", "i-samuel"},
	{"2 samuel", "ii-samuel"},
	{"2 sam", "ii-samuel"},
	{"2sam", "ii-samuel"},
	{"2samuel", "ii-samuel"},
	{"second samuel", "ii-samuel"},
	{"שמואל ב", "ii-samuel"},
	{"1 kings", "i-kings"},
	{"1 kgs", "i-kings"},
	{"1kgs", "i-kings"},
	{"1kings", "i-kings"},
	{"first kings", "i-kings"},
	{"מלכים א", "i-kings"},
	{"2 kings", "ii-kings"},
	{"2 kgs", "ii-kings"},
	{"2kgs", "ii-kings"},
	{"2kings", "ii-kings"},
	{"second kings", "ii-kings"},
	{"מלכים ב", "ii-kings"},
	{"isa", "isaiah"},
	{"yeshayahu", "isaiah"},
	{"ישעיה", "isaiah"},
	{"ישעיהו", "isaiah"},
	{"jer", "jeremiah"},
	{"yirmeyahu", "jeremiah"},
	{"ירמיה", "jeremiah"},
	{"ירמיהו", "jeremiah"},
	{"ezek", "ezekiel"},
	{"yechezkel", "ezekiel"},
	{"יחזקאל", "ezekiel"},
	{"hos", "hosea"},
	{"הושע", "hosea"},
	{"יואל", "joel"},
	{"עמוס", "amos"},
	{"obad", "obadiah"},
	{"עובדיה", "obadiah"},
	{"yonah", "jonah"},
	{"יונה", "jonah"},
	{"mic", "micah"},
	{"מיכה", "micah"},
	{"nah", "nahum"},
	{"נחום", "nahum"},
	{"hab", "habakkuk"},
	{"חבקוק", "habakkuk"},
	{"zeph", "zephaniah"},
	{"צפניה", "zephaniah"},
	{"hag", "haggai"},
	{"חגי", "haggai"},
	{"zech", "zechariah"},
	{"זכריה", "zechariah"},
	{"mal", "malachi"},
	{"מלאכי", "malachi"},
	{"ps", "psalms"},
	{"psa", "psalms"},
	{"psalm", "psalms"},
	{"tehillim", "psalms"},
	{"תהלים", "psalms"},
	{"תהילים", "psalms"},
	{"prov", "proverbs"},
	{"mishlei", "proverbs"},
	{"משלי", "proverbs"},
	{"iyov", "job"},
	{"איוב", "job"},
	{"song of solomon", "song-of-songs"},
	{"canticles", "song-of-songs"},
	{"shir hashirim", "song-of-songs"},
	{"שיר השירים", "song-of-songs"},
	{"רות", "ruth"},
	{"lam", "lamentations"},
	{"eicha", "lamentations"},
	{"איכה", "lamentations"},
	{"eccl", "ecclesiastes"},
	{"qohelet", "ecclesiastes"},
	{"qoheleth", "ecclesiastes"},
	{"kohelet", "ecclesiastes"},
	{"קהלת", "ecclesiastes"},
	{"אסתר", "esther"},
	{"dan", "daniel"},
	{"דניאל", "daniel"},
	{"עזרא", "ezra"},
	{"neh", "nehemiah"},
	{"נחמיה", "nehemiah"},
	{"1 chronicles", "i-chronicles"},
	{"1 chr", "i-chronicles"},
	{"1chr", "i-chronicles"},
	{"first chronicles", "i-chronicles"},
	{"דברי הימים א", "i-chronicles"},
	{"2 chronicles", "ii-chronicles"},
	{"2 chr", "ii-chronicles"},
	{"2chr", "ii-chronicles"},
	{"second chronicles", "ii-chronicles"},
	{"דברי הימים ב", "ii-chronicles"},
}

var (
	bookAlias map[string]string
	bookOrder map[string]int
)

func init() {
	if len(tanakhCanon) != 39 {
		panic("db: tanakh canon is not 39 books")
	}
	bookAlias = make(map[string]string, len(tanakhCanon)*2+len(tanakhExtraAliases))
	bookOrder = make(map[string]int, len(tanakhCanon))
	sum := 0
	seenSlug := map[string]bool{}
	for i, b := range tanakhCanon {
		if seenSlug[b.Slug] {
			panic("db: duplicate tanakh slug " + b.Slug)
		}
		seenSlug[b.Slug] = true
		sum += b.Verses
		bookOrder[b.Slug] = i
		addBookAlias(b.Slug, b.Slug)
		addBookAlias(b.Title, b.Slug)
	}
	if sum != 23206 {
		panic("db: tanakh canon verse count is not 23206")
	}
	for _, extra := range tanakhExtraAliases {
		if !seenSlug[extra.slug] {
			panic("db: tanakh alias for unknown book " + extra.slug)
		}
		addBookAlias(extra.alias, extra.slug)
	}
}

func addBookAlias(alias, slug string) {
	key := normBook(alias)
	if key == "" {
		panic("db: empty tanakh alias")
	}
	if prev, ok := bookAlias[key]; ok && prev != slug {
		panic("db: tanakh alias " + key + " is " + prev + " and " + slug)
	}
	bookAlias[key] = slug
}

// normBook folds a book name to the alias key: lower case, hyphens and
// periods become spaces, and the words are joined by one space.
func normBook(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '-', '_', '.', '/':
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// TanakhBooks returns the 39 books in canonical order.
// The slice is a copy. Verses is the pinned public-domain count.
func TanakhBooks() []TanakhBook {
	out := make([]TanakhBook, len(tanakhCanon))
	copy(out, tanakhCanon)
	return out
}

// CanonicalBook resolves a book name to its slug.
// It accepts the slug, the English title, and a few common aliases
// (Gen, 1 Samuel, Bereshit, בראשית, Ps). ok is false when the name is unknown.
func CanonicalBook(name string) (slug string, ok bool) {
	slug, ok = bookAlias[normBook(name)]
	return slug, ok
}

func tanakhRank(slug string) int {
	if n, ok := bookOrder[slug]; ok {
		return n
	}
	return len(tanakhCanon)
}
