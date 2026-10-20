package main

// Curated placement items.
//
// Band 0 keeps the tutor worksheet words. Bands 1–4 add niqqud to the
// FrequencyWords token named by List, which is the word at Rank (1-based).
// Pointing is ktiv male with niqqud, including the mater yod in אִישָּׁה.
// Three forms omit a mater the subtitle list writes: לְסַיֵּם / לסיים,
// עָיֵף / עייף, שֻׁלְחָן / שולחן. ktivMale in main.go pairs those.
// Gender is m, f, both, or empty. Verbs and adjectives have none.
// Transliteration uses kh, ts, and '. These rows are checked, so Verified
// is true.
//
// SPEC-006 grew every band to 40 words. Each added row is built with
// tutorP or freqP, whose last argument is the Pealim dictionary path that
// confirmed the pointing, part of speech, gender, and gloss (see NOTICE).
// Band 0 additions come from the tutor's class lexicon and worksheet word
// list. Four of them are present-tense forms the class uses (גָּר, רוֹצֶה,
// יוֹצֵא, מַמְלִיץ); their path is the verb's page, where the form is listed.
// Bands 1–4 additions use the Pealim headword, which is ktiv male, so they
// need no ktivMale entry. A word Pealim could not confirm is not added.

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

func tutorP(id, text, gloss, pos, translit, gender, pealim string) rawItem {
	it := tutor(id, text, gloss, pos, translit, gender)
	it.Pealim = pealim
	return it
}

func freqP(id, text, gloss, pos, translit, gender string, band, rank int, list, pealim string) rawItem {
	it := freq(id, text, gloss, pos, translit, gender, band, rank, list)
	it.Pealim = pealim
	return it
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
		tutorP("te", "תֵּה", "tea", "noun", "te", "m", "/dict/6873-te/"),
		tutorP("ben", "בֵּן", "son", "noun", "ben", "m", "/dict/2665-ben/"),
		tutorP("bat", "בַּת", "daughter", "noun", "bat", "f", "/dict/2666-bat/"),
		tutorP("shem", "שֵׁם", "name", "noun", "shem", "m", "/dict/3194-shem/"),
		tutorP("cheshbon", "חֶשְׁבּוֹן", "bill", "noun", "kheshbon", "m", "/dict/3594-cheshbon/"),
		tutorP("slicha", "סְלִיחָה", "pardon", "noun", "slikha", "f", "/dict/3996-slicha/"),
		tutorP("raayon", "רַעֲיוֹן", "idea", "noun", "ra'ayon", "m", "/dict/5050-raayon/"),
		tutorP("teva", "טֶבַע", "nature", "noun", "teva", "m", "/dict/4522-teva/"),
		tutorP("tov", "טוֹב", "good", "adjective", "tov", "", "/dict/5422-tov/"),
		tutorP("ken", "כֵּן", "yes", "adverb", "ken", "", "/dict/4529-ken/"),
		tutorP("kama", "כַּמָּה", "how much", "adverb", "kama", "", "/dict/4784-kama/"),
		tutorP("eifo", "אֵיפֹה", "where", "adverb", "eifo", "", "/dict/5805-eifo/"),
		tutorP("gar", "גָּר", "lives", "verb", "gar", "", "/dict/4-lagur/"),
		tutorP("rotze", "רוֹצֶה", "wants", "verb", "rotse", "", "/dict/12-lirtzot/"),
		tutorP("yotze", "יוֹצֵא", "goes out", "verb", "yotse", "", "/dict/832-latzet/"),
		tutorP("lenasot", "לְנַסּוֹת", "to try", "verb", "lenasot", "", "/dict/1210-lenasot/"),
		tutorP("lehachin", "לְהָכִין", "to prepare", "verb", "lehakhin", "", "/dict/893-lehachin/"),
		tutorP("noach", "נוֹחַ", "comfortable", "adjective", "noakh", "", "/dict/4470-noach/"),
		tutorP("mamlitz", "מַמְלִיץ", "recommends", "verb", "mamlits", "", "/dict/1061-lehamlitz/"),
		tutorP("keday", "כְּדַאי", "worthwhile", "adverb", "keday", "", "/dict/5648-keday/"),

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
		freq("isha", "אִישָּׁה", "woman", "noun", "isha", "f", 1, 409, "אישה"),
		freq("achshav", "עַכְשָׁיו", "now", "adverb", "akhshav", "", 1, 41, "עכשיו"),
		freq("tamid", "תָּמִיד", "always", "adverb", "tamid", "", 1, 147, "תמיד"),
		freq("kesef", "כֶּסֶף", "money", "noun", "kesef", "m", 1, 269, "כסף"),
		freq("ish", "אִישׁ", "man", "noun", "ish", "m", 1, 348, "איש"),
		freqP("shana", "שָׁנָה", "year", "noun", "shana", "f", 1, 364, "שנה", "/dict/4065-shana/"),
		freqP("zman", "זְמַן", "time", "noun", "zman", "m", 1, 95, "זמן", "/dict/3055-zman/"),
		freqP("adam", "אָדָם", "person", "noun", "adam", "m", 1, 176, "אדם", "/dict/3553-adam/"),
		freqP("derech", "דֶּרֶךְ", "way", "noun", "derekh", "f", 1, 183, "דרך", "/dict/6077-derech/"),
		freqP("lev", "לֵב", "heart", "noun", "lev", "m", 1, 341, "לב", "/dict/4353-lev/"),
		freqP("avoda", "עֲבוֹדָה", "work", "noun", "avoda", "f", 1, 270, "עבודה", "/dict/3140-avoda/"),
		freqP("beaya", "בְּעָיָה", "problem", "noun", "be'aya", "f", 1, 316, "בעיה", "/dict/4383-beaya/"),
		freqP("sof", "סוֹף", "end", "noun", "sof", "m", 1, 324, "סוף", "/dict/5543-sof/"),
		freqP("ladaat", "לָדַעַת", "to know", "verb", "lada'at", "", 1, 197, "לדעת", "/dict/791-ladaat/"),
		freqP("lomar", "לוֹמַר", "to say", "verb", "lomar", "", 1, 213, "לומר", "/dict/55-lomar/"),
		freqP("latet", "לָתֵת", "to give", "verb", "latet", "", 1, 255, "לתת", "/dict/1285-latet/"),
		freqP("lavo", "לָבוֹא", "to come", "verb", "lavo", "", 1, 378, "לבוא", "/dict/28-lavo/"),
		freqP("ledaber", "לְדַבֵּר", "to speak", "verb", "ledaber", "", 1, 122, "לדבר", "/dict/2-ledaber/"),
		freqP("lachshov", "לַחְשׁוֹב", "to think", "verb", "lakhshov", "", 1, 379, "לחשוב", "/dict/700-lachshov/"),
		freqP("ra", "רַע", "bad", "adjective", "ra", "", 1, 317, "רע", "/dict/4244-ra/"),
		freqP("acher", "אַחֵר", "other", "adjective", "akher", "", 1, 143, "אחר", "/dict/4630-acher/"),
		freqP("kan", "כָּאן", "here", "adverb", "kan", "", 1, 35, "כאן", "/dict/3191-kan/"),
		freqP("meod", "מְאוֹד", "very", "adverb", "me'od", "", 1, 85, "מאוד", "/dict/5449-meod/"),
		freqP("od", "עוֹד", "more", "adverb", "od", "", 1, 80, "עוד", "/dict/5241-od/"),
		freqP("shuv", "שׁוּב", "again", "adverb", "shuv", "", 1, 124, "שוב", "/dict/4335-shuv/"),

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
		freqP("mila", "מִילָּה", "word", "noun", "mila", "f", 2, 893, "מילה", "/dict/4746-mila/"),
		freqP("sheela", "שְׁאֵלָה", "question", "noun", "she'ela", "f", 2, 647, "שאלה", "/dict/2843-sheela/"),
		freqP("chag", "חַג", "holiday", "noun", "khag", "m", 2, 874, "חג", "/dict/4868-chag/"),
		freqP("yayin", "יַיִן", "wine", "noun", "yayin", "m", 2, 683, "יין", "/dict/5960-yayin/"),
		freqP("matsav", "מַצָּב", "situation", "noun", "matsav", "m", 2, 725, "מצב", "/dict/2638-matzav/"),
		freqP("sipur", "סִיפּוּר", "story", "noun", "sipur", "m", 2, 676, "סיפור", "/dict/2981-sipur/"),
		freqP("siba", "סִיבָּה", "reason", "noun", "siba", "f", 2, 631, "סיבה", "/dict/4268-siba/"),
		freqP("shoter", "שׁוֹטֵר", "police officer", "noun", "shoter", "m", 2, 902, "שוטר", "/dict/4664-shoter/"),
		freqP("lechakot", "לְחַכּוֹת", "to wait", "verb", "lekhakot", "", 2, 643, "לחכות", "/dict/594-lechakot/"),
		freqP("lesachek", "לְשַׂחֵק", "to play", "verb", "lesakhek", "", 2, 602, "לשחק", "/dict/2172-lesachek/"),
		freqP("leshalem", "לְשַׁלֵּם", "to pay", "verb", "leshalem", "", 2, 851, "לשלם", "/dict/2232-leshalem/"),
		freqP("lishon", "לִישׁוֹן", "to sleep", "verb", "lishon", "", 2, 618, "לישון", "/dict/860-lishon/"),
		freqP("liknot", "לִקְנוֹת", "to buy", "verb", "liknot", "", 2, 717, "לקנות", "/dict/1913-liknot/"),
		freqP("likro", "לִקְרוֹא", "to read", "verb", "likro", "", 2, 581, "לקרוא", "/dict/13-likro/"),
		freqP("lehatchil", "לְהַתְחִיל", "to start", "verb", "lehatkhil", "", 2, 611, "להתחיל", "/dict/2359-lehatchil/"),
		freqP("rishon", "רִאשׁוֹן", "first", "adjective", "rishon", "", 2, 715, "ראשון", "/dict/3400-rishon/"),
		freqP("amiti", "אֲמִיתִּי", "real", "adjective", "amiti", "", 2, 542, "אמיתי", "/dict/8678-amiti/"),
		freqP("matzchik", "מַצְחִיק", "funny", "adjective", "matskhik", "", 2, 572, "מצחיק", "/dict/8092-matzchik/"),
		freqP("male", "מָלֵא", "full", "adjective", "male", "", 2, 699, "מלא", "/dict/4396-male/"),
		freqP("bekarov", "בְּקָרוֹב", "soon", "adverb", "bekarov", "", 2, 629, "בקרוב", "/dict/7695-bekarov/"),

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
		freqP("milchama", "מִלְחָמָה", "war", "noun", "milkhama", "f", 3, 1269, "מלחמה", "/dict/3704-milchama/"),
		freqP("or", "אוֹר", "light", "noun", "or", "m", 3, 1352, "אור", "/dict/5232-or/"),
		freqP("chalom", "חָלוֹם", "dream", "noun", "khalom", "m", 3, 1695, "חלום", "/dict/3127-chalom/"),
		freqP("michtav", "מִכְתָּב", "letter", "noun", "mikhtav", "m", 3, 1942, "מכתב", "/dict/3035-michtav/"),
		freqP("shir", "שִׁיר", "song", "noun", "shir", "m", 3, 1538, "שיר", "/dict/4337-shir/"),
		freqP("tmuna", "תְּמוּנָה", "picture", "noun", "tmuna", "f", 3, 1179, "תמונה", "/dict/4260-tmuna/"),
		freqP("kochav", "כּוֹכָב", "star", "noun", "kokhav", "m", 3, 1554, "כוכב", "/dict/4054-kochav/"),
		freqP("chodesh", "חוֹדֶשׁ", "month", "noun", "khodesh", "m", 3, 1687, "חודש", "/dict/3208-chodesh/"),
		freqP("linsoa", "לִנְסוֹעַ", "to travel", "verb", "linso'a", "", 3, 1218, "לנסוע", "/dict/1215-linsoa/"),
		freqP("lichtov", "לִכְתּוֹב", "to write", "verb", "likhtov", "", 3, 1307, "לכתוב", "/dict/1-lichtov/"),
		freqP("lishloach", "לִשְׁלוֹחַ", "to send", "verb", "lishlo'akh", "", 3, 1107, "לשלוח", "/dict/2220-lishloach/"),
		freqP("larutz", "לָרוּץ", "to run", "verb", "laruts", "", 3, 1599, "לרוץ", "/dict/11-larutz/"),
		freqP("lishkoach", "לִשְׁכּוֹחַ", "to forget", "verb", "lishkoakh", "", 3, 1750, "לשכוח", "/dict/36-lishkoach/"),
		freqP("lizkor", "לִזְכּוֹר", "to remember", "verb", "lizkor", "", 3, 1927, "לזכור", "/dict/34-lizkor/"),
		freqP("livnot", "לִבְנוֹת", "to build", "verb", "livnot", "", 3, 1323, "לבנות", "/dict/234-livnot/"),
		freqP("atsuv", "עָצוּב", "sad", "adjective", "atsuv", "", 3, 1597, "עצוב", "/dict/4208-atzuv/"),
		freqP("adom", "אָדוֹם", "red", "adjective", "adom", "", 3, 1596, "אדום", "/dict/3487-adom/"),
		freqP("yakar", "יָקָר", "expensive", "adjective", "yakar", "", 3, 1795, "יקר", "/dict/3331-yakar/"),
		freqP("matok", "מָתוֹק", "sweet", "adjective", "matok", "", 3, 1035, "מתוק", "/dict/4618-matok/"),
		freqP("pitom", "פִּתְאוֹם", "suddenly", "adverb", "pit'om", "", 3, 1414, "פתאום", "/dict/6523-pitom/"),

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
		freqP("mafteach", "מַפְתֵּחַ", "key", "noun", "mafte'akh", "m", 4, 2253, "מפתח", "/dict/3521-mafteach/"),
		freqP("rakevet", "רַכֶּבֶת", "train", "noun", "rakevet", "f", 4, 2708, "רכבת", "/dict/3300-rakevet/"),
		freqP("bakbuk", "בַּקְבּוּק", "bottle", "noun", "bakbuk", "m", 4, 2565, "בקבוק", "/dict/4730-bakbuk/"),
		freqP("etsba", "אֶצְבַּע", "finger", "noun", "etsba", "f", 4, 3081, "אצבע", "/dict/4871-etzba/"),
		freqP("sakin", "סַכִּין", "knife", "noun", "sakin", "m", 4, 2471, "סכין", "/dict/5939-sakin/"),
		freqP("dag", "דָּג", "fish", "noun", "dag", "m", 4, 2533, "דג", "/dict/4787-dag/"),
		freqP("geshem", "גֶּשֶׁם", "rain", "noun", "geshem", "m", 4, 2615, "גשם", "/dict/6090-geshem/"),
		freqP("uga", "עוּגָה", "cake", "noun", "uga", "f", 4, 3432, "עוגה", "/dict/5375-uga/"),
		freqP("litzchok", "לִצְחוֹק", "to laugh", "verb", "litskhok", "", 4, 3228, "לצחוק", "/dict/1785-litzchok/"),
		freqP("livkot", "לִבְכּוֹת", "to cry", "verb", "livkot", "", 4, 2374, "לבכות", "/dict/17-livkot/"),
		freqP("likpotz", "לִקְפּוֹץ", "to jump", "verb", "likpots", "", 4, 2089, "לקפוץ", "/dict/1931-likpotz/"),
		freqP("letzalem", "לְצַלֵּם", "to photograph", "verb", "letsalem", "", 4, 2516, "לצלם", "/dict/1802-letzalem/"),
		freqP("lishbor", "לִשְׁבּוֹר", "to break", "verb", "lishbor", "", 4, 2184, "לשבור", "/dict/2135-lishbor/"),
		freqP("lanuach", "לָנוּחַ", "to rest", "verb", "lanuakh", "", 4, 2482, "לנוח", "/dict/1164-lanuach/"),
		freqP("latus", "לָטוּס", "to fly", "verb", "latus", "", 4, 3666, "לטוס", "/dict/740-latus/"),
		freqP("amitz", "אַמִּיץ", "brave", "adjective", "amits", "", 4, 3347, "אמיץ", "/dict/3317-amitz/"),
		freqP("yarok", "יָרוֹק", "green", "adjective", "yarok", "", 4, 3008, "ירוק", "/dict/3479-yarok/"),
		freqP("kachol", "כָּחוֹל", "blue", "adjective", "kakhol", "", 4, 2675, "כחול", "/dict/3477-kachol/"),
		freqP("ashir", "עָשִׁיר", "rich", "adjective", "ashir", "", 4, 2412, "עשיר", "/dict/4884-ashir/"),
		freqP("chalash", "חַלָּשׁ", "weak", "adjective", "khalash", "", 4, 2863, "חלש", "/dict/3348-chalash/"),
	}
}
