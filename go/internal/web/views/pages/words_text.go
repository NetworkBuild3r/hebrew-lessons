package pages

// wordsSyncHeader asks the handler to return a fresh filter bar out of band.
const wordsSyncHeader = `{"X-Words-Sync":"1"}`

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func weakValue(value string) string {
	if value == "" || value == "0" {
		return "1"
	}
	return value
}

func strengthHe(bucket string) string {
	he, _ := strengthLabel(bucket)
	return he
}

func strengthEn(bucket string) string {
	_, en := strengthLabel(bucket)
	return en
}

func errHe(kind string) string {
	if kind == "invalid_filter" {
		return "הַסִּנּוּן הַזֶּה לֹא תָּקֵף"
	}
	return "לֹא הִצְלַחְנוּ לִטְעֹן אֶת הַמִּלִּים"
}

func errEn(kind string) string {
	if kind == "invalid_filter" {
		return "That filter is not valid"
	}
	return "We couldn’t load your words"
}
