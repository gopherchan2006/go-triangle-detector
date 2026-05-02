package main

// DetectOption configures a single call to DetectAscendingTriangle.
type DetectOption func(*detectOpts)

type detectOpts struct {
	params  DetectorParams
	trace   bool
	counter RejectCounter
}

func newDetectOpts(opts []DetectOption) detectOpts {
	o := detectOpts{
		params:  DefaultDetectorParams(),
		trace:   true,
		counter: NoopCounter{},
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// WithTrace controls whether verbose log strings are collected (default: true).
// Set false in hot-path scanning to avoid expensive string formatting.
func WithTrace(on bool) DetectOption {
	return func(o *detectOpts) { o.trace = on }
}

// WithParams overrides the detector thresholds.
func WithParams(p DetectorParams) DetectOption {
	return func(o *detectOpts) { o.params = p }
}

// WithCounter sets a RejectCounter for tracking filter hits.
func WithCounter(c RejectCounter) DetectOption {
	return func(o *detectOpts) { o.counter = c }
}
