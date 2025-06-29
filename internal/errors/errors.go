package customerrors

import "errors"

var (
	ErrInvalidLanguageFormat   = errors.New("invalid language format")
	ErrInvalidVisibilityFormat = errors.New("invalid visibility format")
	ErrInvalidExpirationFormat = errors.New("invalid expiration format")

	ErrFailedFetchPassword = errors.New("failed to fetch password hash")
	ErrPasswordRequired    = errors.New("password is required")

	ErrPastaNotFound   = errors.New("pasta is not found")
	ErrNoAccess        = errors.New("no access, private pasta")
	ErrPasswordIsEmpty = errors.New("password is empty")
	ErrWrongPassword   = errors.New("password is wrong")

	// redis
	ErrKeyDoesntExist = errors.New("key doesn't exitst")
)
