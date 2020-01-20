package errs

import (
	"github.com/twitchtv/twirp"
)

// InternalError describes an error interface to use internally for the service.
type InternalError interface {
	error

	Code() InternalErrorCode

	WrappedError() error
}

// ConvertToExternalError Will attempt to convert to an external Twirp error.
func ConvertToExternalError(e InternalError) error {
	if e == nil {
		return nil
	}

	if err, ok := e.(twirp.Error); ok {
		return err
	}

	switch e.Code() {
	case IncorrectInformation:
		fallthrough
	case IncorrectLoginProfile:
		return twirp.NewError(twirp.InvalidArgument, "incorrect information")
	case Forbidden:
		fallthrough
	case AccessTokenRevoked:
		return twirp.NewError(twirp.Unauthenticated, "forbidden")
	case NotFound:
		return twirp.NewError(twirp.NotFound, "not found")
	case Unknown:
		fallthrough
	case DownstreamError:
		return twirp.NewError(twirp.Unknown, "unknown error")
	}

	return twirp.InternalErrorWith(e)
}