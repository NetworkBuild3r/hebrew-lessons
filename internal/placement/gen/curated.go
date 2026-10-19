package main

// Curated placement items.
//
// Band 0 keeps the tutor worksheet pointing. Bands 1–4 add niqqud to the
// FrequencyWords token named by List, which is the word at Rank (1-based).
// Three dictionary spellings omit a mater the subtitle list writes:
// לְסַיֵּם / לסיים, עָיֵף / עייף, שֻׁלְחָן / שולחן.
// Pseudo-words are invented roots in ordinary mishkal patterns.

func bandPtr(n int) *int {
	v := n
	return &v
}

func tutor(id, text, gloss, pos string) rawItem {
	return rawItem{ID: id, Text: text, Band: bandPtr(0), Gloss: gloss, POS: pos, Source: "tutor"}
}

func freq(id, text, gloss, pos string, band, rank int, list string) rawItem {
	return rawItem{ID: id, Text: text, Band: bandPtr(band), Gloss: gloss, POS: pos, Source: "freq", Rank: rank, List: list}
}

func pseudo(id, text, pos string) rawItem {
	return rawItem{ID: id, Text: text, Pseudo: true, POS: pos, Source: "pseudo"}
}

func curatedItems() []rawItem {
	return []rawItem{
		tutor("boker", "בֹּקֶר", "morning", "noun"),
		tutor("kafe", "קָפֶה", "coffee", "noun"),
		tutor("seret", "סֶרֶט", "movie", "noun"),
		tutor("marak", "מָרָק", "soup", "noun"),
		tutor("misrad", "מִשְׂרָד", "office", "noun"),
		tutor("ohel", "אֹהֶל", "tent", "noun"),
		tutor("kam", "קָם", "gets up", "verb"),
		tutor("shote", "שׁוֹתֶה", "drinks", "verb"),
		tutor("yodea", "יוֹדֵעַ", "knows", "verb"),
		tutor("levashel", "לְבַשֵּׁל", "to cook", "verb"),
		tutor("meyuchad", "מְיוּחָד", "special", "adjective"),
		tutor("batuach", "בָּטוּחַ", "sure", "adjective"),

		freq("nachon", "נָכוֹן", "correct", "adjective", 1, 52, "נכון"),
		freq("laasot", "לַעֲשׂוֹת", "to do", "verb", 1, 53, "לעשות"),
		freq("davar", "דָּבָר", "thing", "noun", 1, 78, "דבר"),
		freq("lirot", "לִרְאוֹת", "to see", "verb", 1, 109, "לראות"),
		freq("lalechet", "לָלֶכֶת", "to go", "verb", 1, 115, "ללכת"),
		freq("ohev", "אוֹהֵב", "loves", "verb", 1, 141, "אוהב"),
		freq("yom", "יוֹם", "day", "noun", 1, 149, "יום"),
		freq("chaver", "חָבֵר", "friend", "noun", 1, 150, "חבר"),
		freq("makom", "מָקוֹם", "place", "noun", 1, 199, "מקום"),
		freq("layla", "לַיְלָה", "night", "noun", 1, 360, "לילה"),
		freq("katan", "קָטָן", "small", "adjective", 1, 371, "קטן"),
		freq("chadash", "חָדָשׁ", "new", "adjective", 1, 423, "חדש"),

		freq("leechol", "לֶאֱכוֹל", "to eat", "verb", 2, 512, "לאכול"),
		freq("lishmor", "לִשְׁמוֹר", "to keep", "verb", 2, 516, "לשמור"),
		freq("lehavin", "לְהָבִין", "to understand", "verb", 2, 527, "להבין"),
		freq("kal", "קַל", "easy", "adjective", 2, 521, "קל"),
		freq("chazak", "חָזָק", "strong", "adjective", 2, 529, "חזק"),
		freq("rosh", "רֹאשׁ", "head", "noun", 2, 551, "ראש"),
		freq("yalda", "יַלְדָּה", "girl", "noun", 2, 569, "ילדה"),
		freq("shaa", "שָׁעָה", "hour", "noun", 2, 623, "שעה"),
		freq("mayim", "מַיִם", "water", "noun", 2, 678, "מים"),
		freq("cheder", "חֶדֶר", "room", "noun", 2, 711, "חדר"),
		freq("rachok", "רָחוֹק", "far", "adjective", 2, 889, "רחוק"),
		freq("kelev", "כֶּלֶב", "dog", "noun", 2, 1000, "כלב"),

		freq("laamod", "לַעֲמוֹד", "to stand", "verb", 3, 1007, "לעמוד"),
		freq("tsair", "צָעִיר", "young", "adjective", 3, 1012, "צעיר"),
		freq("gavoah", "גָּבוֹהַּ", "tall", "adjective", 3, 1017, "גבוה"),
		freq("tinok", "תִּינוֹק", "baby", "noun", 3, 1044, "תינוק"),
		freq("lesayem", "לְסַיֵּם", "to finish", "verb", 3, 1061, "לסיים"),
		freq("liftoach", "לִפְתּוֹחַ", "to open", "verb", 3, 1066, "לפתוח"),
		freq("mechonit", "מְכוֹנִית", "car", "noun", 3, 1112, "מכונית"),
		freq("aroch", "אָרוֹךְ", "long", "adjective", 3, 1213, "ארוך"),
		freq("olam", "עוֹלָם", "world", "noun", 3, 1370, "עולם"),
		freq("delet", "דֶּלֶת", "door", "noun", 3, 1606, "דלת"),
		freq("ets", "עֵץ", "tree", "noun", 3, 1671, "עץ"),
		freq("ir", "עִיר", "city", "noun", 3, 1998, "עיר"),

		freq("sachkan", "שַׂחְקָן", "actor", "noun", 4, 2013, "שחקן"),
		freq("linhog", "לִנְהוֹג", "to drive", "verb", 4, 2030, "לנהוג"),
		freq("tshuva", "תְּשׁוּבָה", "answer", "noun", 4, 2032, "תשובה"),
		freq("tseva", "צֶבַע", "color", "noun", 4, 2043, "צבע"),
		freq("lashir", "לָשִׁיר", "to sing", "verb", 4, 2041, "לשיר"),
		freq("lehachlit", "לְהַחְלִיט", "to decide", "verb", 4, 2057, "להחליט"),
		freq("taim", "טָעִים", "tasty", "adjective", 4, 2064, "טעים"),
		freq("panuy", "פָּנוּי", "available", "adjective", 4, 2146, "פנוי"),
		freq("ayef", "עָיֵף", "tired", "adjective", 4, 2294, "עייף"),
		freq("shulchan", "שֻׁלְחָן", "table", "noun", 4, 2303, "שולחן"),
		freq("shaon", "שָׁעוֹן", "watch", "noun", 4, 3156, "שעון"),
		freq("shemesh", "שֶׁמֶשׁ", "sun", "noun", 4, 3495, "שמש"),

		pseudo("gazaf", "גָּזַף", "verb"),
		pseudo("zafat", "זָפַט", "verb"),
		pseudo("gasak", "גָּסַק", "verb"),
		pseudo("kazar", "כָּזַר", "verb"),
		pseudo("gozef", "גּוֹזֵף", "verb"),
		pseudo("sozem", "סּוֹזֵם", "verb"),
		pseudo("higzif", "הִגְזִיף", "verb"),
		pseudo("hikzim", "הִקְזִים", "verb"),
		pseudo("gizuf", "גִּיזּוּף", "noun"),
		pseudo("ziput", "זִיפּוּט", "noun"),
		pseudo("migzaf", "מִגְזָף", "noun"),
		pseudo("miszam", "מִסְזָם", "noun"),
		pseudo("gazfan", "גַּזְפָן", "noun"),
		pseudo("sazman", "סַזְמָן", "noun"),
		pseudo("tsivus", "צִיבּוּס", "noun"),
		pseudo("kizum", "קִיזּוּם", "noun"),
		pseudo("gazif", "גָּזִיף", "adjective"),
		pseudo("kazim", "קָזִים", "adjective"),
		pseudo("sazim", "סָזִים", "adjective"),
		pseudo("tsavis", "צָבִיס", "adjective"),
	}
}
