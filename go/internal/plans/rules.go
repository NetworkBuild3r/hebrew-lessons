package plans

import "github.com/NetworkBuild3r/hebrew-lessons/go/internal/jsstr"

func checkLessonRules(plan Value, lessons []lessonEntry, ctx valContext, f *findings) {
	mig, _ := plan.Get("migration")
	legacy := false
	if mig.IsObject() {
		if leg, ok := mig.Get("legacy"); ok && leg.K == KindBool && leg.B {
			legacy = true
		}
	}
	for i := range lessons {
		entry := &lessons[i]
		checkNewWordCap(entry, legacy, f)
		checkRecycles(entry, f)
		checkIntroductions(entry, f)
		checkFluency(entry, f)
		checkMeanings(entry, f)
		checkPoints(entry, f)
		checkAnswers(entry, f)
		checkBlockInforms(entry, f)
		checkPictureOnRecall(entry, f)
		checkClosedSet(entry, f)
		if ctx.v120 {
			checkExerciseBlocks(entry, f)
		}
		if !ctx.v110 {
			continue
		}
		checkConjugationTables(entry, ctx, f)
		checkExerciseCount(entry, f)
		checkReviewFirst(entry, f)
		checkPersonalQuestion(entry, f)
		checkVerseSources(entry, f)
	}
	checkUnits(plan, ctx, f)
	checkCheckpoints(lessons, f)
	if ctx.v110 {
		checkRecipeRuns(lessons, f)
	}
	if aud, ok := plan.Get("audience"); ok && aud.K == KindStr && aud.S == "personal" {
		f.add("queue-order", "", "a personal queue may insert due reviews; it keeps this plan's lesson order", "")
	}
}

func checkExerciseCount(entry *lessonEntry, f *findings) {
	if entry.recipe != "topic-conversation" {
		return
	}
	count := 0
	for _, item := range entry.blocks {
		if item.typ == "exercise" {
			count++
		}
	}
	if count > 2 {
		f.add("N23", entry.at+"/blocks",
			strconvI(count)+" exercise blocks; a topic-conversation lesson warns above 2", "")
	}
}

func checkPictureOnRecall(entry *lessonEntry, f *findings) {
	for _, item := range entry.blocks {
		if item.typ != "type-it" || !typeItHasPictureCue(item.block) {
			continue
		}
		f.add("picture-on-recall", item.at, "a Type it cue is a picture; hide the picture on recall", "")
	}
}

func typeItHasPictureCue(block Value) bool {
	if !block.IsObject() {
		return false
	}
	if img, ok := block.Get("image"); ok && jsTruthy(img) {
		return true
	}
	if pic, ok := block.Get("picture"); ok && jsTruthy(pic) {
		return true
	}
	if id, ok := block.Get("mediaId"); ok && id.K == KindStr && id.S != "" {
		return true
	}
	return false
}

func jsTruthy(v Value) bool {
	switch v.K {
	case KindNull:
		return false
	case KindBool:
		return v.B
	case KindNum:
		return v.N != 0 && isFinite(v.N)
	case KindStr:
		return v.S != ""
	default:
		return true
	}
}

func checkClosedSet(entry *lessonEntry, f *findings) {
	if len(entry.newWords) == 0 {
		return
	}
	for _, word := range entry.newWords {
		if _, ok := kinshipStripped[strippedKey(word.raw)]; !ok {
			return
		}
	}
	f.add("closed-set", entry.at+"/newWords", "every new word is a kinship term; mix people, places, and actions", "")
}

func checkNewWordCap(entry *lessonEntry, legacy bool, f *findings) {
	count := len(entry.newWords) + len(entry.verseWords)
	if entry.recipe == "topic-conversation" {
		checkTopicConversation(entry, count, legacy, f)
		return
	}
	if count <= 8 {
		return
	}
	msg := strconvI(count) + " new words; N1 allows 8"
	sev := "block"
	if legacy {
		msg += " (migration.legacy: warn only)"
		sev = "warn"
	}
	f.add("N1", entry.at+"/newWords", msg, sev)
}

func checkTopicConversation(entry *lessonEntry, count int, legacy bool, f *findings) {
	if count > 15 {
		msg := strconvI(count) + " new words; a topic-conversation lesson blocks above 15"
		sev := "block"
		if legacy {
			msg += " (migration.legacy: warn only)"
			sev = "warn"
		}
		f.add("N18", entry.at+"/newWords", msg, sev)
	} else if count > 8 {
		f.add("N18", entry.at+"/newWords", strconvI(count)+" new words; a topic-conversation lesson warns above 8", "")
	}
	for _, item := range entry.blocks {
		if item.typ != "introduction" {
			continue
		}
		sizes := map[int]int{}
		words, _ := item.block.Get("words")
		for w, word := range listOf(words) {
			if !word.IsObject() {
				continue
			}
			g, ok := word.Get("group")
			if !ok || g.K != KindNum || !isJSInteger(g.N) {
				continue
			}
			group := int(g.N)
			sizes[group]++
			if sizes[group] == 6 {
				f.add("N18", item.at+"/words/"+strconvI(w), "introduction group "+numString(g.N)+" has more than 5 words", "")
			}
		}
	}
}

func checkRecycles(entry *lessonEntry, f *findings) {
	if entry.recipe == "guide" {
		return
	}
	builds, _ := entry.lesson.Get("buildsOn")
	if len(listOf(builds)) == 0 || len(entry.recycled) >= 3 {
		return
	}
	f.add("N2", entry.at+"/recycles", "buildsOn is set and "+strconvI(len(entry.recycled))+" words are recycled; N2 asks for 3", "")
}

func checkIntroductions(entry *lessonEntry, f *findings) {
	n := 0
	for _, item := range entry.blocks {
		if item.typ != "introduction" {
			continue
		}
		n++
		if n == 1 {
			continue
		}
		msg := "a second introduction; N5 allows one per lesson"
		if pat, ok := item.block.Get("pattern"); ok && pat.IsObject() {
			msg = "a second introduction, with a second pattern; N5 allows one of each"
		}
		f.add("N5", item.at, msg, "")
	}
}

func checkFluency(entry *lessonEntry, f *findings) {
	for _, item := range entry.blocks {
		if item.typ != "fluency-pass" {
			continue
		}
		for _, key := range []string{"words", "newWords"} {
			if item.block.Has(key) {
				f.add("N12", item.at+"/"+key, msgFluencyList, "")
			}
		}
		if len(entry.newWords) > 0 {
			f.add("N12", item.at, "a fluency-pass in a lesson that introduces new words", "warn")
		}
	}
}

func checkMeanings(entry *lessonEntry, f *findings) {
	glossed := map[string]struct{}{}
	for _, item := range entry.blocks {
		if item.typ != "introduction" {
			continue
		}
		words, _ := item.block.Get("words")
		for _, word := range listOf(words) {
			if !word.IsObject() {
				continue
			}
			w, wok := word.Get("word")
			m, mok := word.Get("meaning")
			if wok && w.K == KindStr && mok && m.K == KindStr && jsstr.Trim(m.S) != "" {
				glossed[exactKey(w.S)] = struct{}{}
			}
		}
	}
	for _, word := range entry.newWords {
		if _, ok := glossed[word.key]; !ok {
			f.add("meaning", word.at, "no introduction word gives this new word a first-language meaning", "")
		}
	}
}

func checkPoints(entry *lessonEntry, f *findings) {
	words := append(append([]wordRef{}, entry.newWords...), entry.verseWords...)
	for _, word := range words {
		if !hasHebrewPoint(word.raw) {
			f.add("pointed-word", word.at, "the new word has no Hebrew vowel point", "")
		}
	}
}

func checkExerciseBlocks(entry *lessonEntry, f *findings) {
	schema := loadSchema()
	for _, item := range entry.blocks {
		if item.typ != "exercise" {
			continue
		}
		kind := ""
		if k, ok := item.block.Get("kind"); ok && k.K == KindStr {
			kind = k.S
		}
		items, _ := item.block.Get("items")
		for i, row := range listOf(items) {
			if !row.IsObject() {
				continue
			}
			at := item.at + "/items/" + strconvI(i)
			switch kind {
			case "choose":
				ans, aok := row.Get("answer")
				choices, cok := row.Get("choices")
				if aok && ans.K == KindStr && cok && choices.K == KindArr {
					found := false
					for _, choice := range choices.A {
						if choice.K == KindStr && choice.S == ans.S {
							found = true
							break
						}
					}
					if !found {
						f.add("N20", at+"/answer", "the answer is not one of this item's choices", "")
					}
				}
			case "unscramble":
				words := stringList(row, "words")
				if len(words) < 3 || len(words) > 10 {
					f.add("N21", at+"/words", "unscramble needs 3 to 10 words", "")
				}
				alts, _ := row.Get("alt")
				for a, order := range listOf(alts) {
					if order.K != KindArr {
						continue
					}
					altAt := at + "/alt/" + strconvI(a)
					altWords := []string{}
					for _, token := range order.A {
						if token.K == KindStr {
							altWords = append(altWords, token.S)
						}
					}
					if len(altWords) < 3 || len(altWords) > 10 {
						f.add("N21", altAt, "each alternate order needs 3 to 10 words", "")
						continue
					}
					if !sameWordMultiset(words, altWords) {
						f.add("N21", altAt, "this order is not a rearrangement of the same words", "")
					} else if sameWordOrder(words, altWords) {
						f.add("N21", altAt, "an alternate order must not match the main word order", "")
					}
				}
			case "dictation":
				ap, ok := row.Get("audioPath")
				if !ok || ap.K != KindStr || ap.S == "" {
					f.add("N22", at+"/audioPath", "dictation needs an audio path", "")
				} else if !schema.audioPath.MatchString(ap.S) {
					f.add("N22", at+"/audioPath", "audio path must be a relative .mp3 path with no scheme or leading slash", "")
				}
			}
			acc, _ := row.Get("accept")
			for a, alt := range listOf(acc) {
				if alt.K == KindStr && hasHebrewPoint(alt.S) {
					f.add("schema", at+"/accept/"+strconvI(a), "accept must not include vowel points or cantillation", "")
				}
			}
		}
	}
}

func stringList(row Value, key string) []string {
	arr, _ := row.Get(key)
	var out []string
	for _, token := range listOf(arr) {
		if token.K == KindStr {
			out = append(out, token.S)
		}
	}
	return out
}

func sameWordOrder(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func sameWordMultiset(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	tally := func(words []string) map[string]int {
		m := map[string]int{}
		for _, w := range words {
			m[w]++
		}
		return m
	}
	a, b := tally(left), tally(right)
	if len(a) != len(b) {
		return false
	}
	for w, c := range a {
		if b[w] != c {
			return false
		}
	}
	return true
}

func checkAnswers(entry *lessonEntry, f *findings) {
	for _, item := range entry.blocks {
		if item.typ != "introduction" {
			continue
		}
		cloze, _ := item.block.Get("cloze")
		if !cloze.IsObject() {
			cloze = VObj()
		}
		items, _ := cloze.Get("items")
		for i, clozeItem := range listOf(items) {
			if clozeItem.IsObject() {
				if ans, ok := clozeItem.Get("answer"); ok && ans.K == KindStr && !hasHebrewLetter(ans.S) {
					f.add("romanized-answer", item.at+"/cloze/items/"+strconvI(i)+"/answer", msgRomanized, "")
				}
			}
		}
		pat, _ := item.block.Get("pattern")
		if !pat.IsObject() {
			pat = VObj()
		}
		pitems, _ := pat.Get("items")
		for i, patternItem := range listOf(pitems) {
			if !patternItem.IsObject() {
				continue
			}
			ans, aok := patternItem.Get("answer")
			choices, cok := patternItem.Get("choices")
			if !aok || ans.K != KindNum || !isJSInteger(ans.N) || !cok || choices.K != KindArr {
				continue
			}
			at := item.at + "/pattern/items/" + strconvI(i)
			n := int(ans.N)
			if n < 0 || n >= len(choices.A) {
				f.add("romanized-answer", at+"/answer", "the answer index names no choice, so there is no Hebrew answer", "")
			} else if choices.A[n].K == KindStr && !hasHebrewLetter(choices.A[n].S) {
				f.add("romanized-answer", at+"/choices/"+strconvI(n), msgRomanized, "")
			}
		}
	}
}

func checkBlockInforms(entry *lessonEntry, f *findings) {
	for _, item := range entry.blocks {
		if item.typ == "graded-story" || item.typ == "listening" {
			qs, _ := item.block.Get("questions")
			if len(listOf(qs)) > 0 {
				f.add("questions", item.at+"/questions", "comprehension questions are optional and are not the recall event", "")
			}
		}
		if item.typ == "spaced-review-checkpoint" {
			f.add("recall-day", item.at, "the scheduler places this recall on a later day; the plan holds no dates", "")
		}
	}
}

func checkConjugationTables(entry *lessonEntry, ctx valContext, f *findings) {
	for _, item := range entry.blocks {
		if item.typ != "conjugation-table" {
			continue
		}
		table := item.block
		lemma := ""
		hasLemma := false
		if lk, ok := table.Get("lemmaKey"); ok && lk.K == KindStr {
			lemma = exactKey(lk.S)
			hasLemma = true
		}
		seen := map[string]struct{}{}
		cells, _ := table.Get("cells")
		for i, row := range listOf(cells) {
			if !row.IsObject() {
				continue
			}
			at := item.at + "/cells/" + strconvI(i)
			if cell, ok := row.Get("cell"); ok && cell.K == KindStr {
				if _, dup := seen[cell.S]; dup {
					f.add("N13", at+"/cell", "this cell appears more than once in the table", "")
				}
				seen[cell.S] = struct{}{}
			}
			checkFormKey(row, table, lemma, hasLemma, at+"/formKey", ctx, f)
		}
	}
}

func checkFormKey(row, table Value, lemma string, hasLemma bool, at string, ctx valContext, f *findings) {
	key, ok := row.Get("formKey")
	if !ok || key.K != KindStr || jsstr.CodePoints(key.S) > ctx.keyLimit || !hasHebrewLetter(key.S) {
		return
	}
	if !ctx.lexAvailable {
		f.add("N13", at, "no lexicon was given, so this form cannot be checked", "")
		return
	}
	form := ctx.lex.find(key.S)
	kind, _ := form.Get("kind")
	if form.K == KindNull || kind.K != KindStr || kind.S != "form" {
		f.add("N13", at, "formKey names no stored form", "")
		return
	}
	ver, _ := form.Get("verified")
	if !isVerified(ver) {
		f.add("N14", at, "this form is not verified", "")
	}
	if lemmaMismatch(form, lemma, hasLemma) {
		f.add("N13", at, "formKey names a form of another lemma", "")
		return
	}
	tense, hasTense := table.Get("tense")
	if propMismatch(form, "tense", tense, hasTense) {
		f.add("N13", at, "formKey names a form of another tense", "")
		return
	}
	cell, hasCell := row.Get("cell")
	if propMismatch(form, "cell", cell, hasCell) {
		f.add("N13", at, "formKey names a form of another cell", "")
	}
}

func isVerified(v Value) bool {
	if v.K == KindBool && v.B {
		return true
	}
	return v.K == KindNum && v.N == 1
}

func lemmaMismatch(form Value, lemma string, hasLemma bool) bool {
	field, ok := form.Get("lemma_key")
	if !ok {
		return true
	}
	if !hasLemma {
		return field.K != KindNull
	}
	return field.K != KindStr || field.S != lemma
}

func propMismatch(form Value, key string, want Value, hasWant bool) bool {
	field, ok := form.Get(key)
	if !ok {
		return true
	}
	if !hasWant {
		return field.K != KindNull
	}
	return !jsEqual(field, want)
}

func checkReviewFirst(entry *lessonEntry, f *findings) {
	if entry.recipe == "" || entry.recipe == "guide" {
		return
	}
	builds, _ := entry.lesson.Get("buildsOn")
	if len(listOf(builds)) == 0 {
		return
	}
	if len(entry.blocks) > 0 && entry.blocks[0].typ == "spaced-review-checkpoint" {
		return
	}
	at := entry.at + "/blocks"
	if len(entry.blocks) > 0 {
		at = entry.blocks[0].at
	}
	f.add("N15", at, "a lesson that builds on earlier lessons starts with a spaced-review-checkpoint", "")
}

func checkPersonalQuestion(entry *lessonEntry, f *findings) {
	switch entry.recipe {
	case "topic-conversation", "verb-workshop", "verse-reading":
	default:
		return
	}
	asked := false
	for _, item := range entry.blocks {
		if item.typ == "personal-question" || item.typ == "assignment" {
			asked = true
			break
		}
		if entry.recipe == "verse-reading" && item.typ == "verse-reading" {
			if pq, ok := item.block.Get("personalQuestion"); ok && pq.IsObject() {
				asked = true
				break
			}
		}
	}
	if !asked {
		f.add("N16", entry.at+"/blocks", "a "+entry.recipe+" lesson has no personal question", "")
	}
}

func checkVerseSources(entry *lessonEntry, f *findings) {
	for _, item := range entry.blocks {
		if item.typ != "verse-reading" {
			continue
		}
		ref, _ := item.block.Get("ref")
		var source Value
		if ref.IsObject() {
			source, _ = ref.Get("source")
		}
		if source.K != KindStr || jsstr.Trim(source.S) == "" {
			f.add("N19", item.at+"/ref/source", "a verse-reading needs a non-empty ref.source naming its edition", "")
		}
	}
}

func checkRecipeRuns(lessons []lessonEntry, f *findings) {
	unit := -1
	recipe := ""
	haveRecipe := false
	run := 0
	for _, entry := range lessons {
		if entry.u != unit {
			unit = entry.u
			recipe = ""
			haveRecipe = false
			run = 0
		}
		if entry.recipe != "" && haveRecipe && entry.recipe == recipe {
			run++
		} else {
			run = 1
		}
		recipe = entry.recipe
		haveRecipe = true
		if recipe != "" && run == 3 {
			f.add("N17", entry.at+"/recipe", "3 lessons in a row in this unit have the recipe "+recipe, "")
		}
	}
}

func checkUnits(plan Value, ctx valContext, f *findings) {
	focused := map[string]struct{}{"introduction": {}}
	label := "introductions"
	if ctx.v110 {
		focused = map[string]struct{}{
			"introduction": {}, "explainer": {}, "conjugation-table": {}, "letter-intro": {},
		}
		label = "language-focused"
	}
	units, _ := plan.Get("units")
	for u, unit := range listOf(units) {
		if !unit.IsObject() {
			continue
		}
		introductions, blocks := 0, 0
		lessons, _ := unit.Get("lessons")
		for _, lesson := range listOf(lessons) {
			if !lesson.IsObject() {
				continue
			}
			recipe := recipeOf(lesson, ctx)
			if recipe == "guide" || recipe == "letters-and-vowels" {
				continue
			}
			bs, _ := lesson.Get("blocks")
			for _, block := range listOf(bs) {
				blocks++
				if block.IsObject() {
					if t, ok := block.Get("type"); ok && t.K == KindStr {
						if _, ok := focused[t.S]; ok {
							introductions++
						}
					}
				}
			}
		}
		if introductions*4 > blocks {
			f.add("N7", "/units/"+strconvI(u), strconvI(introductions)+" of "+strconvI(blocks)+" blocks are "+label+"; N7 allows one in 4", "")
		}
		if band, ok := unit.Get("frequencyBand"); ok && band.K == KindStr && jsstr.Trim(band.S) != "" {
			f.add("frequency-band", "/units/"+strconvI(u)+"/frequencyBand", "frequencyBand is recorded and not checked", "")
		}
	}
}

func checkCheckpoints(lessons []lessonEntry, f *findings) {
	if len(lessons) < 2 {
		return
	}
	lastListed := map[string]int{}
	for _, entry := range lessons {
		for _, item := range entry.blocks {
			if item.typ != "spaced-review-checkpoint" {
				continue
			}
			keys, _ := item.block.Get("wordKeys")
			for _, key := range listOf(keys) {
				if key.K == KindStr {
					lastListed[exactKey(key.S)] = entry.index
				}
			}
		}
	}
	for _, entry := range lessons {
		for _, word := range entry.newWords {
			listed, ok := lastListed[word.key]
			if !ok || listed <= entry.index {
				f.add("later-checkpoint", word.at, "no later spaced-review-checkpoint lists this word", "")
			}
		}
	}
}
