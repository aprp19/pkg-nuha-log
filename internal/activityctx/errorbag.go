package activityctx

import "context"

type errorBagKey struct{}

// ErrorBag holds error log details captured during a request.
type ErrorBag struct {
	Message string
	Context map[string]interface{}
}

// WithErrorBag attaches a mutable error bag to ctx.
func WithErrorBag(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := BagFromContext(ctx); ok {
		return ctx
	}
	return context.WithValue(ctx, errorBagKey{}, &ErrorBag{})
}

// BagFromContext returns the error bag attached to ctx.
func BagFromContext(ctx context.Context) (*ErrorBag, bool) {
	if ctx == nil {
		return nil, false
	}
	bag, ok := ctx.Value(errorBagKey{}).(*ErrorBag)
	return bag, ok && bag != nil
}

// CurrentBag returns the error bag for the active request scope, if any.
func CurrentBag() (*ErrorBag, bool) {
	return BagFromContext(Current())
}

// CaptureErrorLog stores message and structured fields from logger.Error().Msg().
func CaptureErrorLog(message string, fields map[string]interface{}) {
	bag, ok := CurrentBag()
	if !ok {
		return
	}
	if message != "" {
		bag.Message = message
	}
	if len(fields) == 0 {
		return
	}
	if bag.Context == nil {
		bag.Context = make(map[string]interface{}, len(fields))
	}
	for key, value := range fields {
		if key == "" {
			continue
		}
		bag.Context[key] = value
	}
}
