package validate

var (
	SupportedLanguages    = []string{"plaintext", "python", "javascript", "java", "cpp", "csharp", "ruby", "go", "sql", "markdown", "json", "yaml", "html", "css", "bash"}
	SupportedVisibilities = []string{"public", "private"}
	SupportedTime         = map[string]int{
		"1h": 60 * 60 * 1000,
		"1d": 24 * 60 * 60 * 1000,
		"1w": 7 * 24 * 60 * 60 * 1000,
	}
)

func CheckContains(supported []string, elem string) bool {
	for _, i := range supported {
		if i == elem {
			return true
		}
	}
	return false
}
