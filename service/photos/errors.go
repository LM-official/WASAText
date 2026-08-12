package photos

import "errors"

// The failures of this package that the api layer needs to tell apart from a generic error.
// A handler maps them to a status code with errors.Is, so no filesystem message is ever read outside:
// filesystem specific logic never escapes this package
var (
	// ErrInvalidPhotoId is returned when the id is not a canonical UUID
	ErrInvalidPhotoId = errors.New("invalid photo id")

	// ErrPhotoNotFound is returned when no photo is stored under the given id
	ErrPhotoNotFound = errors.New("photo not found")

	// ErrEmptyPhoto is returned when the uploaded file carries no bytes
	ErrEmptyPhoto = errors.New("empty photo")

	// ErrUnsupportedType is returned when the uploaded file is not an image
	ErrUnsupportedType = errors.New("unsupported photo type")

	// ErrPhotoTooLarge is returned when the uploaded file is over schemas.MaxPhotoBytes
	ErrPhotoTooLarge = errors.New("photo too large")
)
