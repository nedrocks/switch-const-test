package errs

// InternalErrorCode defines an error code type for internal errors.
type InternalErrorCode string

const (
	IncorrectInformation = InternalErrorCode("incorrect_information")

	IncorrectLoginProfile = InternalErrorCode("incorrect_login_profile")

	Forbidden = InternalErrorCode("Forbidden")

	AccessTokenRevoked = InternalErrorCode("access_token_revoked")

	NotFound = InternalErrorCode("not_found")

	Unknown = InternalErrorCode("unknown")

	DownstreamError = InternalErrorCode("downstream_error")

)
