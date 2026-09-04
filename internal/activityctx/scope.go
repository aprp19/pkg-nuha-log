package activityctx

import (
	"context"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

var scopes sync.Map

func goroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	field := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, _ := strconv.ParseUint(field, 10, 64)
	return id
}

// Enter stores ctx as the active request scope for the current goroutine.
func Enter(ctx context.Context) {
	if ctx == nil {
		return
	}
	scopes.Store(goroutineID(), ctx)
}

// Leave clears the active request scope for the current goroutine.
func Leave() {
	scopes.Delete(goroutineID())
}

// Current returns the active request context for the current goroutine, if any.
func Current() context.Context {
	if value, ok := scopes.Load(goroutineID()); ok {
		if ctx, ok := value.(context.Context); ok {
			return ctx
		}
	}
	return nil
}
