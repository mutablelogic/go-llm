package tui

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Opt func(*opts)

type opts struct {
	width  int
	height int
}

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

func SetWidth(width int) Opt {
	return func(opts *opts) {
		if width > 0 {
			opts.width = width
		}
	}
}

func SetHeight(height int) Opt {
	return func(opts *opts) {
		if height > 0 {
			opts.height = height
		}
	}
}
