package api

// localized carries a message already rendered in the caller's language while
// staying transparent to errors.Is and errors.As.
//
// The problem it solves: the upload path returns
//
//	fmt.Errorf("%w：需要 %s，剩余 %s", quota.ErrInsufficient, ...)
//
// and the handler both (a) checks errors.Is(err, quota.ErrInsufficient) to pick
// HTTP 507 and (b) sends err.Error() to the client. Replacing %w with a
// translated string would break the status code; keeping %w splices the
// sentinel's own English text into a Chinese sentence — which is what the API
// has been returning: "insufficient storage quota：需要 2 MB，剩余 1 MB".
//
// Wrapping instead of formatting gives both: Error() is exactly the translated
// sentence, and Unwrap() keeps the sentinel reachable.
type localized struct {
	msg string
	err error
}

func (e *localized) Error() string { return e.msg }
func (e *localized) Unwrap() error { return e.err }

// localize pairs a translated message with the error it stands in for.
func localize(msg string, err error) error { return &localized{msg: msg, err: err} }
