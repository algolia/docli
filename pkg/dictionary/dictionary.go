package dictionary

// Dictionary contains strings with specific spelling or capitalization.
var dictionary = map[string]string{
	"csharp":     "C#",
	"javascript": "JavaScript",
	"js":         "JavaScript",
	"php":        "PHP",
	"ts":         "TypeScript",
	"typescript": "TypeScript",
}

// Translate returns the translated string if it's present in the dictionary, the original otherwise.
func Translate(s string) string {
	if dictWord, ok := dictionary[s]; ok {
		return dictWord
	}

	return s
}
