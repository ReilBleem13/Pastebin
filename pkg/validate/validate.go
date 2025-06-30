package validate

import (
	customerrors "pastebin/internal/errors"
	"pastebin/pkg/dto"
	"time"
)

var (
	SupportedLanguages    = []string{"plaintext", "python", "javascript", "java", "cpp", "csharp", "ruby", "go", "sql", "markdown", "json", "yaml", "html", "css", "bash"}
	SupportedVisibilities = []string{"public", "private"}
	SupportedTime         = map[string]int{
		"1h": 60 * 60 * 1000,
		"1d": 24 * 60 * 60 * 1000,
		"1w": 7 * 24 * 60 * 60 * 1000,
	}
)

const (
	defaultLanguage   = "plaintext"
	defaultVisibility = "public"
	defaultExpiration = "1h"
)

func CheckContains(supported []string, elem string) bool {
	for _, i := range supported {
		if i == elem {
			return true
		}
	}
	return false
}

func ValidRequestCreatePasta(request *dto.RequestCreatePasta) (time.Time, error) {
	if request.Message == "" {
		return time.Time{}, customerrors.ErrTextIsEmpty
	}

	if request.Language != "" {
		if !CheckContains(SupportedLanguages, request.Language) {
			return time.Time{}, customerrors.ErrInvalidLanguageFormat
		}
	} else {
		request.Language = defaultLanguage
	}

	if request.Visibility != "" {
		if !CheckContains(SupportedVisibilities, request.Visibility) {
			return time.Time{}, customerrors.ErrInvalidVisibilityFormat
		}
	} else {
		request.Visibility = defaultVisibility
	}

	var timeExpiration time.Time

	if request.Expiration != "" {
		key, ok := SupportedTime[request.Expiration]
		if !ok {
			return time.Time{}, customerrors.ErrInvalidExpirationFormat
		}
		timeExpiration = time.Now().Add(time.Duration(key) * time.Millisecond)
	} else {
		timeExpiration = time.Now().Add(time.Duration(SupportedTime[defaultExpiration]) * time.Millisecond)
	}
	return timeExpiration, nil
}
