package main

// Curated placement items.
//
// Band 0 keeps the tutor worksheet words. Bands 1–4 add niqqud to the
// FrequencyWords token named by List, which is the word at Rank (1-based).
// Pointing is ktiv male with niqqud. אִשָּׁה has no mater yod. Three other
// forms omit a mater the subtitle list writes: לְסַיֵּם / לסיים,
// עָיֵף / עייף, שֻׁלְחָן / שולחן. ktivMale in main.go pairs those.
// Gender is m, f, both, or empty. Verbs and adjectives have none.
// Transliteration uses kh, ts, and '. These rows are checked, so Verified
// is true.

func bandPtr(n int) *int {
	v := n
	return &v
}

func tutor(id, text, gloss, pos, translit, gender string) rawItem {
	return rawItem{ID: id, Text: text, Band: bandPtr(0), Gloss: gloss, POS: pos, Translit: translit, Gender: gender, Source: "tutor", Verified: true}
}

func freq(id, text, gloss, pos, translit, gender string, band, rank int, list string) rawItem {
	return rawItem{ID: id, Text: text, Band: bandPtr(band), Gloss: gloss, POS: pos, Translit: translit, Gender: gender, Source: "freq", Rank: rank, List: list, Verified: true}
}

func curatedItems() []rawItem {
	return []rawItem{
		tutor("boker", "בֹּקֶר", "morning", "noun", "boker", "m"),
		tutor("kafe", "קָפֶה", "coffee", "noun", "kafe", "m"),
		tutor("seret", "סֶרֶט", "movie", "noun", "seret", "m"),
		tutor("marak", "מָרָק", "soup", "noun", "marak", "m"),
		tutor("misrad", "מִשְׂרָד", "office", "noun", "misrad", "m"),
		tutor("ohel", "אֹהֶל", "tent", "noun", "ohel", "m"),
		tutor("kam", "קָם", "gets up", "verb", "kam", ""),
		tutor("shote", "שׁוֹתֶה", "drinks", "verb", "shote", ""),
		tutor("yodea", "יוֹדֵעַ", "knows", "verb", "yodea", ""),
		tutor("levashel", "לְבַשֵּׁל", "to cook", "verb", "levashel", ""),
		tutor("meyuchad", "מְיוּחָד", "special", "adjective", "meyukhad", ""),
		tutor("batuach", "בָּטוּחַ", "sure", "adjective", "batuakh", ""),
		tutor("shalom", "שָׁלוֹם", "peace", "noun", "shalom", "m"),
		tutor("toda", "תּוֹדָה", "thanks", "noun", "toda", "f"),
		tutor("sefer", "סֵפֶר", "book", "noun", "sefer", "m"),
		tutor("bayit", "בַּיִת", "house", "noun", "bayit", "m"),
		tutor("ima", "אִמָּא", "mother", "noun", "ima", "f"),
		tutor("aba", "אַבָּא", "father", "noun", "aba", "m"),
		tutor("chalon", "חַלּוֹן", "window", "noun", "khalon", "m"),
		tutor("kise", "כִּסֵּא", "chair", "noun", "kise", "m"),

		freq("nachon", "נָכוֹן", "correct", "adjective", "nakhon", "", 1, 52, "נכון"),
		freq("laasot", "לַעֲשׂוֹת", "to do", "verb", "la'asot", "", 1, 53, "לעשות"),
		freq("davar", "דָּבָר", "thing", "noun", "davar", "m", 1, 78, "דבר"),
		freq("lirot", "לִרְאוֹת", "to see", "verb", "lir'ot", "", 1, 109, "לראות"),
		freq("lalechet", "לָלֶכֶת", "to go", "verb", "lalekhet", "", 1, 115, "ללכת"),
		freq("ohev", "אוֹהֵב", "loves", "verb", "ohev", "", 1, 141, "אוהב"),
		freq("yom", "יוֹם", "day", "noun", "yom", "m", 1, 149, "יום"),
		freq("chaver", "חָבֵר", "friend", "noun", "khaver", "m", 1, 150, "חבר"),
		freq("makom", "מָקוֹם", "place", "noun", "makom", "m", 1, 199, "מקום"),
		freq("layla", "לַיְלָה", "night", "noun", "layla", "m", 1, 360, "לילה"),
		freq("katan", "קָטָן", "small", "adjective", "katan", "", 1, 371, "קטן"),
		freq("chadash", "חָדָשׁ", "new", "adjective", "khadash", "", 1, 423, "חדש"),
		freq("gadol", "גָּדוֹל", "big", "adjective", "gadol", "", 1, 225, "גדול"),
		freq("yeled", "יֶלֶד", "boy", "noun", "yeled", "m", 1, 273, "ילד"),
		freq("yafa", "יָפֶה", "beautiful", "adjective", "yafe", "", 1, 235, "יפה"),
		freq("isha", "אִשָּׁה", "woman", "noun", "isha", "f", 1, 409, "אישה"),
		freq("achshav", "עַכְשָׁיו", "now", "adverb", "akhshav", "", 1, 41, "עכשיו"),
		freq("tamid", "תָּמִיד", "always", "adverb", "tamid", "", 1, 147, "תמיד"),
		freq("kesef", "כֶּסֶף", "money", "noun", "kesef", "m", 1, 269, "כסף"),
		freq("ish", "אִישׁ", "man", "noun", "ish", "m", 1, 348, "איש"),

		freq("leechol", "לֶאֱכוֹל", "to eat", "verb", "le'ekhol", "", 2, 512, "לאכול"),
		freq("lishmor", "לִשְׁמוֹר", "to keep", "verb", "lishmor", "", 2, 516, "לשמור"),
		freq("lehavin", "לְהָבִין", "to understand", "verb", "lehavin", "", 2, 527, "להבין"),
		freq("kal", "קַל", "easy", "adjective", "kal", "", 2, 521, "קל"),
		freq("chazak", "חָזָק", "strong", "adjective", "khazak", "", 2, 529, "חזק"),
		freq("rosh", "רֹאשׁ", "head", "noun", "rosh", "m", 2, 551, "ראש"),
		freq("yalda", "יַלְדָּה", "girl", "noun", "yalda", "f", 2, 569, "ילדה"),
		freq("shaa", "שָׁעָה", "hour", "noun", "sha'a", "f", 2, 623, "שעה"),
		freq("mayim", "מַיִם", "water", "noun", "mayim", "m", 2, 678, "מים"),
		freq("cheder", "חֶדֶר", "room", "noun", "kheder", "m", 2, 711, "חדר"),
		freq("rachok", "רָחוֹק", "far", "adjective", "rakhok", "", 2, 889, "רחוק"),
		freq("kelev", "כֶּלֶב", "dog", "noun", "kelev", "m", 2, 1000, "כלב"),
		freq("karov", "קָרוֹב", "near", "adjective", "karov", "", 2, 555, "קרוב"),
		freq("mishpacha", "מִשְׁפָּחָה", "family", "noun", "mishpakha", "f", 2, 661, "משפחה"),
		freq("shavua", "שָׁבוּעַ", "week", "noun", "shavua", "m", 2, 759, "שבוע"),
		freq("ach", "אָח", "brother", "noun", "akh", "m", 2, 926, "אח"),
		freq("yad", "יָד", "hand", "noun", "yad", "f", 2, 993, "יד"),
		freq("yam", "יָם", "sea", "noun", "yam", "m", 2, 774, "ים"),
		freq("telefon", "טֶלֶפוֹן", "phone", "noun", "telefon", "m", 2, 952, "טלפון"),
		freq("lilmod", "לִלְמוֹד", "to learn", "verb", "lilmod", "", 2, 863, "ללמוד"),

		freq("laamod", "לַעֲמוֹד", "to stand", "verb", "la'amod", "", 3, 1007, "לעמוד"),
		freq("tsair", "צָעִיר", "young", "adjective", "tsa'ir", "", 3, 1012, "צעיר"),
		freq("gavoah", "גָּבוֹהַּ", "tall", "adjective", "gavoah", "", 3, 1017, "גבוה"),
		freq("tinok", "תִּינוֹק", "baby", "noun", "tinok", "m", 3, 1044, "תינוק"),
		freq("lesayem", "לְסַיֵּם", "to finish", "verb", "lesayem", "", 3, 1061, "לסיים"),
		freq("liftoach", "לִפְתּוֹחַ", "to open", "verb", "liftoakh", "", 3, 1066, "לפתוח"),
		freq("mechonit", "מְכוֹנִית", "car", "noun", "mekhonit", "f", 3, 1112, "מכונית"),
		freq("aroch", "אָרוֹךְ", "long", "adjective", "arokh", "", 3, 1213, "ארוך"),
		freq("olam", "עוֹלָם", "world", "noun", "olam", "m", 3, 1370, "עולם"),
		freq("delet", "דֶּלֶת", "door", "noun", "delet", "f", 3, 1606, "דלת"),
		freq("ets", "עֵץ", "tree", "noun", "ets", "m", 3, 1671, "עץ"),
		freq("ir", "עִיר", "city", "noun", "ir", "f", 3, 1998, "עיר"),
		freq("daka", "דַּקָּה", "minute", "noun", "daka", "f", 3, 1009, "דקה"),
		freq("rofe", "רוֹפֵא", "doctor", "noun", "rofe", "m", 3, 1091, "רופא"),
		freq("achot", "אָחוֹת", "sister", "noun", "akhot", "f", 3, 1594, "אחות"),
		freq("rechov", "רְחוֹב", "street", "noun", "rekhov", "m", 3, 1945, "רחוב"),
		freq("ayin", "עַיִן", "eye", "noun", "ayin", "f", 3, 1311, "עין"),
		freq("regel", "רֶגֶל", "leg", "noun", "regel", "f", 3, 1709, "רגל"),
		freq("cham", "חַם", "hot", "adjective", "kham", "", 3, 1051, "חם"),
		freq("shachor", "שָׁחוֹר", "black", "adjective", "shakhor", "", 3, 1205, "שחור"),

		freq("sachkan", "שַׂחְקָן", "actor", "noun", "sakhkan", "m", 4, 2013, "שחקן"),
		freq("linhog", "לִנְהוֹג", "to drive", "verb", "linhog", "", 4, 2030, "לנהוג"),
		freq("tshuva", "תְּשׁוּבָה", "answer", "noun", "tshuva", "f", 4, 2032, "תשובה"),
		freq("tseva", "צֶבַע", "color", "noun", "tseva", "m", 4, 2043, "צבע"),
		freq("lashir", "לָשִׁיר", "to sing", "verb", "lashir", "", 4, 2041, "לשיר"),
		freq("lehachlit", "לְהַחְלִיט", "to decide", "verb", "lehakhlit", "", 4, 2057, "להחליט"),
		freq("taim", "טָעִים", "tasty", "adjective", "ta'im", "", 4, 2064, "טעים"),
		freq("panuy", "פָּנוּי", "available", "adjective", "panuy", "", 4, 2146, "פנוי"),
		freq("ayef", "עָיֵף", "tired", "adjective", "ayef", "", 4, 2294, "עייף"),
		freq("shulchan", "שֻׁלְחָן", "table", "noun", "shulkhan", "m", 4, 2303, "שולחן"),
		freq("shaon", "שָׁעוֹן", "watch", "noun", "sha'on", "m", 4, 3156, "שעון"),
		freq("shemesh", "שֶׁמֶשׁ", "sun", "noun", "shemesh", "f", 4, 3495, "שמש"),
		freq("more", "מוֹרֶה", "teacher", "noun", "more", "m", 4, 2291, "מורה"),
		freq("chatul", "חָתוּל", "cat", "noun", "khatul", "m", 4, 2748, "חתול"),
		freq("chalav", "חָלָב", "milk", "noun", "khalav", "m", 4, 2575, "חלב"),
		freq("lechem", "לֶחֶם", "bread", "noun", "lekhem", "m", 4, 3423, "לחם"),
		freq("kova", "כּוֹבַע", "hat", "noun", "kova", "m", 4, 3635, "כובע"),
		freq("machshev", "מַחְשֵׁב", "computer", "noun", "makhshev", "m", 4, 2564, "מחשב"),
		freq("matos", "מָטוֹס", "airplane", "noun", "matos", "m", 4, 2313, "מטוס"),
		freq("gan", "גַּן", "garden", "noun", "gan", "m", 4, 2750, "גן"),
	}
}
