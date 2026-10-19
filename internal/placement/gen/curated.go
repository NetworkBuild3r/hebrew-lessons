package main

// Curated placement items.
//
// Band 0 keeps the tutor worksheet pointing. Bands 1–4 add niqqud to the
// FrequencyWords token named by List, which is the word at Rank (1-based).
// Three dictionary spellings omit a mater the subtitle list writes:
// לְסַיֵּם / לסיים, עָיֵף / עייף, שֻׁלְחָן / שולחן.
// Transliteration and gender are unverified, like the glosses.
// Gender is m, f, or empty when the form does not mark gender.

func bandPtr(n int) *int {
	v := n
	return &v
}

func tutor(id, text, gloss, pos, translit, gender string) rawItem {
	return rawItem{ID: id, Text: text, Band: bandPtr(0), Gloss: gloss, POS: pos, Translit: translit, Gender: gender, Source: "tutor"}
}

func freq(id, text, gloss, pos, translit, gender string, band, rank int, list string) rawItem {
	return rawItem{ID: id, Text: text, Band: bandPtr(band), Gloss: gloss, POS: pos, Translit: translit, Gender: gender, Source: "freq", Rank: rank, List: list}
}

func curatedItems() []rawItem {
	return []rawItem{
		tutor("boker", "בֹּקֶר", "morning", "noun", "boker", "m"),
		tutor("kafe", "קָפֶה", "coffee", "noun", "kafe", "m"),
		tutor("seret", "סֶרֶט", "movie", "noun", "seret", "m"),
		tutor("marak", "מָרָק", "soup", "noun", "marak", "m"),
		tutor("misrad", "מִשְׂרָד", "office", "noun", "misrad", "m"),
		tutor("ohel", "אֹהֶל", "tent", "noun", "ohel", "m"),
		tutor("kam", "קָם", "gets up", "verb", "kam", "m"),
		tutor("shote", "שׁוֹתֶה", "drinks", "verb", "shote", "m"),
		tutor("yodea", "יוֹדֵעַ", "knows", "verb", "yodea", "m"),
		tutor("levashel", "לְבַשֵּׁל", "to cook", "verb", "levashel", ""),
		tutor("meyuchad", "מְיוּחָד", "special", "adjective", "meyuchad", "m"),
		tutor("batuach", "בָּטוּחַ", "sure", "adjective", "batuach", "m"),
		tutor("shalom", "שָׁלוֹם", "hello", "noun", "shalom", "m"),
		tutor("toda", "תּוֹדָה", "thanks", "noun", "toda", "f"),
		tutor("sefer", "סֵפֶר", "book", "noun", "sefer", "m"),
		tutor("bayit", "בַּיִת", "house", "noun", "bayit", "m"),

		freq("nachon", "נָכוֹן", "correct", "adjective", "nachon", "m", 1, 52, "נכון"),
		freq("laasot", "לַעֲשׂוֹת", "to do", "verb", "laasot", "", 1, 53, "לעשות"),
		freq("davar", "דָּבָר", "thing", "noun", "davar", "m", 1, 78, "דבר"),
		freq("lirot", "לִרְאוֹת", "to see", "verb", "lirot", "", 1, 109, "לראות"),
		freq("lalechet", "לָלֶכֶת", "to go", "verb", "lalechet", "", 1, 115, "ללכת"),
		freq("ohev", "אוֹהֵב", "loves", "verb", "ohev", "m", 1, 141, "אוהב"),
		freq("yom", "יוֹם", "day", "noun", "yom", "m", 1, 149, "יום"),
		freq("chaver", "חָבֵר", "friend", "noun", "chaver", "m", 1, 150, "חבר"),
		freq("makom", "מָקוֹם", "place", "noun", "makom", "m", 1, 199, "מקום"),
		freq("layla", "לַיְלָה", "night", "noun", "layla", "m", 1, 360, "לילה"),
		freq("katan", "קָטָן", "small", "adjective", "katan", "m", 1, 371, "קטן"),
		freq("chadash", "חָדָשׁ", "new", "adjective", "chadash", "m", 1, 423, "חדש"),
		freq("gadol", "גָּדוֹל", "big", "adjective", "gadol", "m", 1, 225, "גדול"),
		freq("yeled", "יֶלֶד", "boy", "noun", "yeled", "m", 1, 273, "ילד"),
		freq("yafa", "יָפֶה", "beautiful", "adjective", "yafe", "m", 1, 235, "יפה"),
		freq("isha", "אִישָּׁה", "woman", "noun", "isha", "f", 1, 409, "אישה"),

		freq("leechol", "לֶאֱכוֹל", "to eat", "verb", "leechol", "", 2, 512, "לאכול"),
		freq("lishmor", "לִשְׁמוֹר", "to keep", "verb", "lishmor", "", 2, 516, "לשמור"),
		freq("lehavin", "לְהָבִין", "to understand", "verb", "lehavin", "", 2, 527, "להבין"),
		freq("kal", "קַל", "easy", "adjective", "kal", "m", 2, 521, "קל"),
		freq("chazak", "חָזָק", "strong", "adjective", "chazak", "m", 2, 529, "חזק"),
		freq("rosh", "רֹאשׁ", "head", "noun", "rosh", "m", 2, 551, "ראש"),
		freq("yalda", "יַלְדָּה", "girl", "noun", "yalda", "f", 2, 569, "ילדה"),
		freq("shaa", "שָׁעָה", "hour", "noun", "shaa", "f", 2, 623, "שעה"),
		freq("mayim", "מַיִם", "water", "noun", "mayim", "m", 2, 678, "מים"),
		freq("cheder", "חֶדֶר", "room", "noun", "cheder", "m", 2, 711, "חדר"),
		freq("rachok", "רָחוֹק", "far", "adjective", "rachok", "m", 2, 889, "רחוק"),
		freq("kelev", "כֶּלֶב", "dog", "noun", "kelev", "m", 2, 1000, "כלב"),
		freq("karov", "קָרוֹב", "near", "adjective", "karov", "m", 2, 555, "קרוב"),
		freq("mishpacha", "מִשְׁפָּחָה", "family", "noun", "mishpacha", "f", 2, 661, "משפחה"),
		freq("shavua", "שָׁבוּעַ", "week", "noun", "shavua", "m", 2, 759, "שבוע"),
		freq("ach", "אָח", "brother", "noun", "ach", "m", 2, 926, "אח"),

		freq("laamod", "לַעֲמוֹד", "to stand", "verb", "laamod", "", 3, 1007, "לעמוד"),
		freq("tsair", "צָעִיר", "young", "adjective", "tsair", "m", 3, 1012, "צעיר"),
		freq("gavoah", "גָּבוֹהַּ", "tall", "adjective", "gavoah", "m", 3, 1017, "גבוה"),
		freq("tinok", "תִּינוֹק", "baby", "noun", "tinok", "m", 3, 1044, "תינוק"),
		freq("lesayem", "לְסַיֵּם", "to finish", "verb", "lesayem", "", 3, 1061, "לסיים"),
		freq("liftoach", "לִפְתּוֹחַ", "to open", "verb", "liftoach", "", 3, 1066, "לפתוח"),
		freq("mechonit", "מְכוֹנִית", "car", "noun", "mechonit", "f", 3, 1112, "מכונית"),
		freq("aroch", "אָרוֹךְ", "long", "adjective", "aroch", "m", 3, 1213, "ארוך"),
		freq("olam", "עוֹלָם", "world", "noun", "olam", "m", 3, 1370, "עולם"),
		freq("delet", "דֶּלֶת", "door", "noun", "delet", "f", 3, 1606, "דלת"),
		freq("ets", "עֵץ", "tree", "noun", "ets", "m", 3, 1671, "עץ"),
		freq("ir", "עִיר", "city", "noun", "ir", "f", 3, 1998, "עיר"),
		freq("daka", "דַּקָּה", "minute", "noun", "daka", "f", 3, 1009, "דקה"),
		freq("rofe", "רוֹפֵא", "doctor", "noun", "rofe", "m", 3, 1091, "רופא"),
		freq("achot", "אָחוֹת", "sister", "noun", "achot", "f", 3, 1594, "אחות"),
		freq("rechov", "רְחוֹב", "street", "noun", "rechov", "m", 3, 1945, "רחוב"),

		freq("sachkan", "שַׂחְקָן", "actor", "noun", "sachkan", "m", 4, 2013, "שחקן"),
		freq("linhog", "לִנְהוֹג", "to drive", "verb", "linhog", "", 4, 2030, "לנהוג"),
		freq("tshuva", "תְּשׁוּבָה", "answer", "noun", "tshuva", "f", 4, 2032, "תשובה"),
		freq("tseva", "צֶבַע", "color", "noun", "tseva", "m", 4, 2043, "צבע"),
		freq("lashir", "לָשִׁיר", "to sing", "verb", "lashir", "", 4, 2041, "לשיר"),
		freq("lehachlit", "לְהַחְלִיט", "to decide", "verb", "lehachlit", "", 4, 2057, "להחליט"),
		freq("taim", "טָעִים", "tasty", "adjective", "taim", "m", 4, 2064, "טעים"),
		freq("panuy", "פָּנוּי", "available", "adjective", "panuy", "m", 4, 2146, "פנוי"),
		freq("ayef", "עָיֵף", "tired", "adjective", "ayef", "m", 4, 2294, "עייף"),
		freq("shulchan", "שֻׁלְחָן", "table", "noun", "shulchan", "m", 4, 2303, "שולחן"),
		freq("shaon", "שָׁעוֹן", "watch", "noun", "shaon", "m", 4, 3156, "שעון"),
		freq("shemesh", "שֶׁמֶשׁ", "sun", "noun", "shemesh", "f", 4, 3495, "שמש"),
		freq("more", "מוֹרֶה", "teacher", "noun", "more", "m", 4, 2291, "מורה"),
		freq("chatul", "חָתוּל", "cat", "noun", "chatul", "m", 4, 2748, "חתול"),
		freq("chalav", "חָלָב", "milk", "noun", "chalav", "m", 4, 2575, "חלב"),
		freq("lechem", "לֶחֶם", "bread", "noun", "lechem", "m", 4, 3423, "לחם"),
	}
}
