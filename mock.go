package hot

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

	setFlag(fl, true)

	// There can be some point in time when the flag is set
	// but the mock does not yet exist.
	//
	// Also, when we delete the mock there can be brief period of time
	// when mock does not exist but the function already checked the flag
	// and tries to execute the mock anyway.
	//
	// This is why the rewritten code basically looks like this:
	//
	// if atomic.LoadInt32(<flag for the specific function>) != 0 {
	//   // <--- the mock can be deleted from the table at this point in time
	//   if soft := hot.GetMockFor(<function pointer>); soft != nil {
	//	   <execute the mock>
	//     return
	//   }
	// }

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
		return
	}

	// The order doesn't really matter here as the flag is just a hint
	// that the function is mocked and that it's required to check the
	// mocks table whether or not there is an actual mock for it or not.
	//
	// Even if we set the flag to false in before deleting it from the
	// mocks map the actual code that is executed can check for the flag
	// (and see that it's true), then get descheduled and then return
	// to the execution much later and try to get the mock that no longer
	// exists.
	//
	// So in the interceptor code there are two checks:
	//  1. Check that the flag is true
	//  2. Get the mock while holding the mutex and check once
	//     again that it's still there.

	setFlag(fl, false)

	mocksMutex.Lock()
	defer mocksMutex.Unlock()

	delete(mocks, fHash)
}

// Reset removes the mock that was set up for the function f,
// returning it to the original implementation.
// If there were no mocks set up for the function it is a noop.
func Reset(f interface{}) {
	reset(getFuncPtr(f))
}

// ResetByName removes the mock that was set up for the function src,
// returning it to the original implementation.
// If there were no mocks set up for the function it is a noop.
func ResetByName(src string) {
	ptr, ok := pkgPtrs[src]
	if !ok {
		return
	}
	reset(ptr)
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
