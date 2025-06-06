package dictionary

import "testing"

func TestTranslate(t *testing.T) {
	tests := []struct {
		name     string
		word     string
		expected string
	}{
		{
			name:     "Found word in dictionary",
			word:     "csharp",
			expected: "C#",
		},
		{
			name:     "Capitalized word should not be translated",
			word:     "Csharp",
			expected: "Csharp",
		},
		{
			name:     "Word not in dictionary",
			word:     "python",
			expected: "python",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Translate(tt.word)

			if got != tt.expected {
				t.Errorf("Error in translate: got %s, expected %s", got, tt.expected)
			}
		})
	}
}
