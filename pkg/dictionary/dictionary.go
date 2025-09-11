package dictionary

// dictionary contains strings with specific spelling or capitalization.
var dictionary = map[string]string{
	"csharp":     "C#",
	"javascript": "JavaScript",
	"js":         "JavaScript",
	"php":        "PHP",
	"ts":         "TypeScript",
	"typescript": "TypeScript",
}

// normalizedLanguages associates language strings with normalized ones.
var normalizedLanguages = map[string]string{
	"typescript": "ts",
	"javascript": "js",
	"csharp":     "cs",
	"cURL":       "sh",
}

// Translate returns the translated string if it's present in the dictionary, the original otherwise.
func Translate(s string) string {
	if dictWord, ok := dictionary[s]; ok {
		return dictWord
	}

	return s
}

// NormalizeLang returns the normalized language string if it's present, the original otherwise.
func NormalizeLang(s string) string {
	if word, ok := normalizedLanguages[s]; ok {
		return word
	}

	return s
}
