package detect

type Option func(*opts)

type opts struct {
	params  Params
	trace   bool
	counter RejectCounter
}

func newOpts(options []Option) opts {
	o := opts{
		params:  DefaultParams(),
		trace:   true,
		counter: NoopCounter{},
	}
	for _, opt := range options {
		opt(&o)
	}
	return o
}

func WithTrace(on bool) Option {
	return func(o *opts) { o.trace = on }
}

func WithParams(p Params) Option {
	return func(o *opts) { o.params = p }
}

func WithCounter(c RejectCounter) Option {
	return func(o *opts) { o.counter = c }
}
