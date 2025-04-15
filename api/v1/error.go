package v1

var (
	ErrSuccess             = newError(0, "Success")
	ErrBadRequest          = newError(400, "InvalidParam")
	ErrUnauthorized        = newError(401, "Unauthorized")
	ErrNotFound            = newError(404, "NotFound")
	ErrForbidden           = newError(403, "Forbidden")
	ErrInternalServerError = newError(500, "InternalServerError")
)
