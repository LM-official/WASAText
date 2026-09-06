/*
Package photos is the middleware between the photo files and the code.
Every photo is one file named with its id, and only this package knows the directory, the file names and the content types:
filesystem specific logic should never escape this package.

A photo is written once and never changed: a new upload gets a new id, so one id always names the same bytes.
That is what lets the api layer cache a photo URL forever.

The database stores the id, this package stores the bytes.
The two are not one transaction, so the order matters: write the file first and save the id afterwards.
A crash in between leaves a file nobody points at (harmless garbage) instead of an id pointing at nothing (a broken photo).
*/
package photos

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/gofrs/uuid"
)

// sniffLen is the number of bytes http.DetectContentType reads to tell the content type
const sniffLen = 512

// Bundle the avatar so a fresh store does not depend on the working directory.
//
//go:embed default-avatar.png
var defaultAvatar []byte

// allowedTypes are the content types a photo may have: anything else is refused on upload
// The type comes from the bytes, never from the file name or from what the client declares,
// so no one can store something that is not an image and have it served back from this host
var allowedTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
}

// Store is the high level interface for the photo files
type Store interface {
	// Save writes the photo read from r and returns the id of the stored photo
	Save(r io.Reader) (schemas.PhotoId, error)

	// Open returns the file of the photo and its content type: the caller closes the file
	Open(id schemas.PhotoId) (*os.File, string, error)

	// Delete removes the file of the photo; a photo already gone is not a failure
	Delete(id schemas.PhotoId) error
}

// storeimpl is the implementation of Store that keeps the photos in a directory
type storeimpl struct {
	dir string
}

// New returns a new instance of Store that keeps the photos in dir, creating dir if missing.
// `dir` is required - an error will be returned if `dir` is empty
func New(dir string) (Store, error) {
	if dir == "" {
		return nil, errors.New("a directory is required when building a photos Store")
	}

	// 0o755 is the default permission for directories
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("error creating the photos directory: %w", err)
	}

	// Seed only a missing avatar; preserve the file in existing photo volumes
	avatarPath := filepath.Join(dir, string(schemas.DefaultPhotoId))
	if _, err := os.Stat(avatarPath); os.IsNotExist(err) {
		if err := os.WriteFile(avatarPath, defaultAvatar, 0o644); err != nil {
			return nil, fmt.Errorf("creating the default photo: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("checking the default photo: %w", err)
	}

	return &storeimpl{
		dir: dir,
	}, nil
}

// path returns the file name of the photo id.
// An id is a canonical UUID, so it holds no '/' and no '.': checking it here is what stops a
// request from walking out of the photos directory, and it is the only check needed
func (s *storeimpl) path(id schemas.PhotoId) (string, error) {
	if err := id.IsValid(); err != nil {
		return "", ErrInvalidPhotoId
	}

	return filepath.Join(s.dir, string(id)), nil
}

// Save writes the photo read from r and returns the id of the stored photo
func (s *storeimpl) Save(r io.Reader) (schemas.PhotoId, error) {
	// The head of the file is what tells the content type
	head := make([]byte, sniffLen)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		// A photo shorter than sniffLen is fine, a broken read is not
		return "", fmt.Errorf("cannot read the photo: %w", err)
	}
	// Trim to the actual size
	head = head[:n]
	if n == 0 {
		return "", ErrEmptyPhoto
	}

	// Refuse anything that is not an image before a single byte is stored
	contentType := http.DetectContentType(head)
	if !allowedTypes[contentType] {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedType, contentType)
	}

	// Write in a temporary file of the same directory:
	// a reader never sees a half written photo, and the rename below is atomic only inside one filesystem
	tmp, err := os.CreateTemp(s.dir, ".tmp-*")
	if err != nil {
		return "", fmt.Errorf("cannot create the temporary photo file: %w", err)
	}

	// Drop the temporary file unless it reaches its final name
	saved := false
	defer func() {
		_ = tmp.Close()
		if !saved {
			_ = os.Remove(tmp.Name())
		}
	}()

	// Read one byte more than the limit:
	// it is what tells a photo that is too large from one that just fits,
	// instead of silently storing a truncated file
	// Glue back the sniffed bytes
	body := io.MultiReader(bytes.NewReader(head), io.LimitReader(r, schemas.MaxPhotoBytes-int64(n)+1))
	written, err := io.Copy(tmp, body)
	if err != nil {
		return "", fmt.Errorf("cannot write the photo: %w", err)
	}
	if written > schemas.MaxPhotoBytes {
		return "", fmt.Errorf("%w: more than %d bytes", ErrPhotoTooLarge, schemas.MaxPhotoBytes)
	}

	// Reach the disk before the rename makes the photo visible under its final name
	if err := tmp.Sync(); err != nil {
		return "", fmt.Errorf("cannot flush the photo: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("cannot close the photo: %w", err)
	}

	// Generate the new id of the photo
	newUUID, err := uuid.NewV4()
	if err != nil {
		return "", fmt.Errorf("cannot generate the photo id: %w", err)
	}
	id := schemas.PhotoId(newUUID.String())

	name, err := s.path(id)
	if err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), name); err != nil {
		return "", fmt.Errorf("cannot store the photo: %w", err)
	}

	// Prevent defer from removing
	saved = true

	return id, nil
}

// Open returns the file of the photo and its content type: the caller closes the file
func (s *storeimpl) Open(id schemas.PhotoId) (*os.File, string, error) {
	name, err := s.path(id)
	if err != nil {
		return nil, "", err
	}

	f, err := os.Open(name)
	if err != nil {
		if os.IsNotExist(err) {
			// No photo under this id
			return nil, "", ErrPhotoNotFound
		}
		return nil, "", fmt.Errorf("cannot open the photo: %w", err)
	}

	// A photo is saved with no extension, so its type comes from its bytes
	head := make([]byte, sniffLen)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		_ = f.Close()
		return nil, "", fmt.Errorf("cannot read the photo: %w", err)
	}
	contentType := http.DetectContentType(head[:n])

	// Rewind: the caller serves the photo from its first byte
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, "", fmt.Errorf("cannot rewind the photo: %w", err)
	}

	// The photo is streamed instead of loaded into memory
	return f, contentType, nil
}

// Delete removes the file of the photo; a photo already gone is not a failure
func (s *storeimpl) Delete(id schemas.PhotoId) error {
	name, err := s.path(id)
	if err != nil {
		return err
	}

	if err := os.Remove(name); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot delete the photo: %w", err)
	}

	return nil
}
