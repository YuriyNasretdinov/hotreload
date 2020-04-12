package soft

import (
	"reflect"
	"sync"
	"sync/atomic"
)

var mocksMutex sync.Mutex
var mocks = make(map[funcPtr]interface{})

// Mock substitutes the src function with dst in runtime.
// In order to pass function pointers to methods you need to write
// the following expression: `(*typeName).MethodName`.
func Mock(src interface{}, dst interface{}) {
	mock(getFuncPtr(src), dst)
}

func mock(fHash funcPtr, dst interface{}) {

	fl, ok := pkgFlags[fHash]
	if !ok {
		panic("Function cannot be mocked, it is not registered")
	}

	if !reflect.TypeOf(dst).ConvertibleTo(reflect.TypeOf(pkgFuncs[fHash])) {
		panic("Function signatures do not match")
	}

	atomic.StoreInt32((*int32)(fl), 1)

	mocksMutex.Lock()
	defer mocksMutex.Unlock()

	mocks[fHash] = dst
}

// MockByName mocks the function by it's name src. Dst is the function that should replace src.
//
// The name of a function is formed using the following scheme:
// 1. For plain functions it's just "package/Symbol",
//       E.g. for `os.Open()` it would be "os/Open"
// 2. For pointer methods it is "package/*receiverType.MethodName":
//       E.g. for `Close` method of `*os.File` it would be "os/*File.Close"
// 3. For value methods it is "package/receiverType.MethodName":
//       E.g. for `IsZero` method of `time.Time` it would be "time/Time.IsZero"
func MockByName(src string, dst interface{}) {
	ptr, ok := pkgPtrs[src]
	if !ok {
		panic("No function with the name `" + src + "` is registered")
	}
	mock(ptr, dst)
}

func getFlag(h flagPtr) bool {
	return atomic.LoadInt32((*int32)(h)) != 0
}

func setFlag(h flagPtr, v bool) {
	vInt := int32(0)
	if v {
		vInt = 1
	}

	atomic.StoreInt32((*int32)(h), vInt)
}

// CallOriginal calls the original implementation of the function f.
// Note that the behaviour of recursive functions is not defined.
func CallOriginal(f interface{}, args ...interface{}) []interface{} {
	return callOriginal(getFuncPtr(f), args)
}

// CallOriginalByName calls the original implementation of the function f.
// Note that the behaviour of recursive functions is not defined.
func CallOriginalByName(f string, args ...interface{}) []interface{} {
	ptr, ok := pkgPtrs[f]
	if !ok {
		panic("No function with the name `" + f + "` is registered")
	}
	return callOriginal(ptr, args)
}

func callOriginal(fHash funcPtr, args []interface{}) []interface{} {
	fl, ok := pkgFlags[fHash]
	if !ok {
		panic("Function is not registered")
	}

	if getFlag(fl) {
		setFlag(fl, false)
		defer setFlag(fl, true)
	}

	in := make([]reflect.Value, 0, len(args))
	for _, arg := range args {
		in = append(in, reflect.ValueOf(arg))
	}

	out := reflect.ValueOf(pkgFuncs[fHash]).Call(in)
	res := make([]interface{}, 0, len(out))
	for _, v := range out {
		res = append(res, v.Interface())
	}

	return res
}

func reset(fHash funcPtr) {
	fl, ok := pkgFlags[fHash]
	if !ok {
		panic("Function is not registered")
	}

	delete(mocks, fHash)
	setFlag(fl, false)
}

// Reset removes the mock that was set up for the function f,
// returning it to the original implementation.
func Reset(f interface{}) {
	reset(getFuncPtr(f))
}

// ResetAll removes the mocks that were set up for all functions.
func ResetAll() {
	for ptr := range mocks {
		reset(ptr)
	}
}

// GetMockFor returns the mock that was set in Mock() method for the supplied
// function if such mock exists and nil otherwise.
func GetMockFor(f interface{}) interface{} {
	fHash := getFuncPtr(f)

	mocksMutex.Lock()
	res := mocks[fHash]
	mocksMutex.Unlock()

	return res
}
