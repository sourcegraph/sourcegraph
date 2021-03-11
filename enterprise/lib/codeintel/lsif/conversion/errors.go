package conversion

import (
	"errors"
	"fmt"
	"strings"
)

// ErrMissingMetaData occurs when no metadata vertex is present or not the first lne in the upload.
var ErrMissingMetaData = errors.New("no metadata defined")

// ErrUnexpectedPayload occurs when the reader does not deserialize the payload of an element
// as expected by the correlation process. This signifies a programming error.
var ErrUnexpectedPayload = errors.New("unexpected payload for element")

// ErrMalformedDump is an error that occurs when the correlator find an identifier
// that does not point to the correct element (if it points to any element at all).
type ErrMalformedDump struct {
	// id is the identifier of the element in which the error occurs.
	id int

	// references is the identifier being referenced by the failing element.
	references int

	// kinds is the type(s) of elements references should refer to.
	kinds []string
}

func (e ErrMalformedDump) Error() string {
	return fmt.Sprintf("unknown reference to %d (expected a %s) in element %d", e.references, strings.Join(e.kinds, " or "), e.id)
}

// malformedDump creates a new ErrMalformedDump error.
func malformedDump(id, references int, kinds ...string) error {
	return ErrMalformedDump{
		id:         id,
		references: references,
		kinds:      kinds,
	}
}
