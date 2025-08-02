package validate

import (
	customerrors "pastebin/internal/errors"
	"pastebin/internal/models"
	"pastebin/pkg/dto"
	"strings"
	"time"
)

var (
	SupportedLanguages = []string{"plaintext", "python", "javascript", "java", "cpp", "csharp", "ruby", "go", "sql", "markdown", "json", "yaml", "html", "css", "bash"}
	SupportedTime      = map[string]int{
		"1h": 60 * 60 * 1000,
		"1d": 24 * 60 * 60 * 1000,
		"1w": 7 * 24 * 60 * 60 * 1000,
	}
)

const (
	defaultLanguage   = "plaintext"
	defaultExpiration = "1h"
	defaultVisibility = models.VisibilityPublic
)

func CheckContains(supported []string, elem string) bool {
	for _, i := range supported {
		if i == elem {
			return true
		}
	}
	return false
}

func ValidRequestCreatePasta(request *dto.RequestCreatePasta) (time.Duration, error) {
	if request.Message == "" {
		return 0, customerrors.ErrTextIsEmpty
	}

	if request.Language != "" {
		if !CheckContains(SupportedLanguages, request.Language) {
			return 0, customerrors.ErrInvalidLanguageFormat
		}
	} else {
		request.Language = defaultLanguage
	}

	if request.Visibility != "" {
		if !models.IsValidVisibility(models.Visibility(request.Visibility)) {
			return 0, customerrors.ErrInvalidVisibilityFormat
		}
	} else {
		request.Visibility = string(defaultVisibility)
	}

	var timeExpiration time.Duration

	if request.Expiration != "" {
		request.Expiration = strings.ToLower(request.Expiration)
		key, ok := SupportedTime[request.Expiration]
		if !ok {
			return 0, customerrors.ErrInvalidExpirationFormat
		}
		timeExpiration = time.Duration(key) * time.Millisecond
	} else {
		timeExpiration = time.Duration(SupportedTime[defaultExpiration]) * time.Millisecond
	}
	return timeExpiration, nil
}
