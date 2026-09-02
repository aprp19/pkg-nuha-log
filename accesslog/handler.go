package accesslog

import (
	"reflect"
	"runtime"
	"strings"
)

func handlerName(h interface{}) string {
	v := reflect.ValueOf(h)
	if v.Kind() != reflect.Func {
		return ""
	}

	name := runtime.FuncForPC(v.Pointer()).Name()
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	return strings.TrimSuffix(name, "-fm")
}
