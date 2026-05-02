package detect

// Option configures a single call to DetectAscendingTriangle.
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

// WithTrace controls whether verbose log strings are collected (default: true).
func WithTrace(on bool) Option {
	return func(o *opts) { o.trace = on }
}

// WithParams overrides the detector thresholds.
func WithParams(p Params) Option {
	return func(o *opts) { o.params = p }
}

// WithCounter sets a RejectCounter for tracking filter hits.
func WithCounter(c RejectCounter) Option {
	return func(o *opts) { o.counter = c }
}
