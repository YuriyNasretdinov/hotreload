package soft

import "reflect"

type (
	funcPtr uintptr
	flagPtr *int32 // flag pointer indicates that there exists a mock and is atomically read and written
)

var pkgFlags = make(map[funcPtr]flagPtr)
var pkgPtrs = make(map[string]funcPtr)
var pkgFuncs = make(map[funcPtr]interface{})

func getFuncPtr(f interface{}) funcPtr {
	return funcPtr(reflect.ValueOf(f).Pointer())
}

// RegisterFunc is a callback that is used in rewritten files to register
// the function so that it can be mocked.
// Do not use directly.
func RegisterFunc(fun interface{}, name string, p *int32) {
	f := getFuncPtr(fun)
	pkgFlags[f] = flagPtr(p)
	pkgFuncs[f] = fun
	pkgPtrs[name] = f
}
