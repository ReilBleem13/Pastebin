package customerrors

import "errors"

var (
	ErrInvalidLanguageFormat   = errors.New("invalid language format")
	ErrInvalidVisibilityFormat = errors.New("invalid visibility format")
	ErrInvalidExpirationFormat = errors.New("invalid expiration format")
	ErrTextIsEmpty             = errors.New("text is empty")
	ErrInvalidQueryParament    = errors.New("invalid query parament")

	ErrFailedFetchPassword = errors.New("failed to fetch password hash")
	ErrPasswordRequired    = errors.New("password is required")

	ErrPastaNotFound   = errors.New("pasta is not found")
	ErrNoAccess        = errors.New("no access, private pasta")
	ErrPasswordIsEmpty = errors.New("password is empty")
	ErrWrongPassword   = errors.New("password is wrong")

	// redis
	ErrKeyDoesntExist = errors.New("key doesn't exitst")

	//middleware
	ErrUserNotAuthenticated = errors.New("user is not authenticated")
	ErrInternal             = errors.New("internal server error")

	// jwt
	ErrUnexpectedSignMethod = errors.New("unexpected signing method")
	ErrTokenExpired         = errors.New("token expired")
	ErrInvalidToken         = errors.New("token is invalid")

	//user
	ErrInvalidEmailFormat = errors.New("invalid email format")
	ErrShortPassword      = errors.New("password is too short. Min lenght = 10")
	ErrUserNotFound       = errors.New("user is not found")
	ErrUserAlreadyExist   = errors.New("user already exists")

	ErrEmptySearch = errors.New("search is empty")
)
