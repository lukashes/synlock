package synlock

type Option func(*options)

type options struct {
	errorHandler func(key string, err error)
}

func WithErrorHandler(h func(key string, err error)) Option {
	return func(o *options) { o.errorHandler = h }
}
