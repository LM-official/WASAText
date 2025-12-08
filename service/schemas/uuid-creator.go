package schemas

import "github.com/gofrs/uuid"

// creates a UUID with a given prefix and returns it as a string
func CreateUUID(prefix string) (string, error) {
	newUUID, err := uuid.NewV4()
	if err != nil {
		return "", err
	}

	// concatenate prefix and UUID
	return prefix + "-" + newUUID.String(), nil
}
