package parallel

// Must panics if it is given a non-nil error.
// Otherwise, it returns the first argument
func Must[T any](result T, err error) T {
	if err != nil {
		panic(err)
	}
	return result
}

// Must0 panics if it is given a non-nil error.
func Must0(err error) {
	if err != nil {
		panic(err)
	}
}
