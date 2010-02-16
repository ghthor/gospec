// Copyright © 2009-2010 Esko Luontola <www.orfjackal.net>
// This software is released under the Apache License 2.0.
// The license text is at http://www.apache.org/licenses/LICENSE-2.0

package gospec

import (
	"fmt"
	"container/vector"
	"exp/iterable"
	"math"
	"os"
	"reflect"
)


type matcherAdapter struct {
	location *Location
	log      errorLogger
}

func newMatcherAdapter(location *Location, log errorLogger) *matcherAdapter {
	return &matcherAdapter{location, log}
}

func (this *matcherAdapter) Expect(actual interface{}, matcher Matcher, expected ...interface{}) {
	ok, pos, _, err := matcher.Match(actual, expected)
	if err != nil {
		this.addError(err.String())
	} else if !ok {
		this.addError(pos.String())
	}
}

func (this *matcherAdapter) addError(message string) {
	e := newError(message, this.location)
	this.log.AddError(e)
}


// Matchers are used in expectations to compare the actual and expected values.
// 
// Return values:
//   ok:  Should be true when `actual` and `expected` match, otherwise false.
//   pos: Message for a failed expectation.
//   neg: Message for a failed expectation when the matcher is combined with Not.
//   err: Message for an unrecoverable error, for example if the arguments had a wrong type.
type Matcher func(actual interface{}, expected interface{}) (ok bool, pos os.Error, neg os.Error, err os.Error)

// Calls the matcher with the actual value and an optional expected value.
// If no expected value is given, then <nil> will be used.
func (matcher Matcher) Match(actual interface{}, optionalExpected ...interface{}) (ok bool, pos os.Error, neg os.Error, err os.Error) {
	var expected interface{}
	if len(optionalExpected) > 0 {
		expected = optionalExpected[0]
	}
	ok, pos, neg, err = matcher(actual, expected)
	return
}


// Constructs an error message the same way as fmt.Sprintf(), but the string is
// created lazily when it is used, if it is used at all. This avoids unnecessary
// string parsing in matchers, because most of the time there are no failures
// and thus the error messages are not used.
func Errorf(format string, args ...interface{}) os.Error {
	return lazyStringer(func() interface{} {
		return fmt.Sprintf(format, args)
	})
}

type lazyStringer func() interface{}

func (this lazyStringer) String() string {
	return fmt.Sprint(this())
}


// Easy array creation, to give multiple expected values to a matcher.
func Values(values ...interface{}) []interface{} {
	return values
}


// Negates the meaning of a Matcher. Matches when the original matcher does not
// match, and the other way around.
func Not(matcher Matcher) Matcher {
	return func(actual interface{}, expected interface{}) (ok bool, pos os.Error, neg os.Error, err os.Error) {
		ok, pos, neg, err = matcher(actual, expected)
		ok = !ok
		pos, neg = neg, pos
		return
	}
}


// The actual value must equal the expected value. For primitives the equality
// operator is used. All other objects must implement the Equality interface.
func Equals(actual interface{}, expected interface{}) (ok bool, pos os.Error, neg os.Error, err os.Error) {
	ok = areEqual(actual, expected)
	// TODO: change the messages to following?
	// '%v' should equal '%v', but it did not
	// '%v' should NOT equal '%v', but it did
	pos = Errorf("Expected '%v' but was '%v'", expected, actual)
	neg = Errorf("Did not expect '%v' but was '%v'", expected, actual)
	return
}

func areEqual(a interface{}, b interface{}) bool {
	if a2, ok := a.(Equality); ok {
		return a2.Equals(b)
	}
	return a == b
}

type Equality interface {
	Equals(other interface{}) bool
}


// The actual value must be a pointer to the same object as the expected value.
func IsSame(actual interface{}, expected interface{}) (ok bool, pos os.Error, neg os.Error, err os.Error) {
	ptr1, err := pointerOf(actual);
	if err != nil{
		return
	}
	ptr2, err := pointerOf(expected)
	if err != nil{
		return
	}
	ok = ptr1 == ptr2
	pos = Errorf("Expected '%v' but was '%v'", expected, actual)
	neg = Errorf("Did not expect '%v' but was '%v'", expected, actual)
	return
}

func pointerOf(value interface{}) (ptr uintptr, err os.Error) {
	switch v := reflect.NewValue(value).(type) {
	case *reflect.PtrValue:
		ptr = v.Get()
	default:
		err = Errorf("Expected a pointer, but was '%v' of type '%T'", value, value)
	}
	return
}


// The actual value must be <nil>, or a typed nil pointer inside an interface value.
// See http://groups.google.com/group/golang-nuts/browse_thread/thread/d900674d491ef8d
// for discussion on how in Go typed nil values can turn into non-nil interface values.
func IsNil(actual interface{}, _ interface{}) (ok bool, pos os.Error, neg os.Error, err os.Error) {
	ok = actual == nil || isNilPointerInsideInterfaceValue(actual)
	pos = Errorf("Expected <nil> but was '%v'", actual)
	neg = Errorf("Did not expect <nil> but was '%v'", actual)
	return
}

func isNilPointerInsideInterfaceValue(value interface{}) bool {
	switch v := reflect.NewValue(value).(type) {
	case *reflect.PtrValue:
		return v.IsNil()
	}
	return false
}


// The actual value must be <true>.
func IsTrue(actual interface{}, _ interface{}) (ok bool, pos os.Error, neg os.Error, err os.Error) {
	ok, pos, neg, err = Equals(actual, true)
	return
}


// The actual value must be <false>.
func IsFalse(actual interface{}, _ interface{}) (ok bool, pos os.Error, neg os.Error, err os.Error) {
	ok, pos, neg, err = Equals(actual, false)
	return
}


// The actual value must satisfy the given criteria.
func Satisfies(actual interface{}, criteria interface{}) (ok bool, pos os.Error, neg os.Error, err os.Error) {
	ok = criteria.(bool) == true
	pos = Errorf("Criteria not satisfied by '%v'", actual)
	neg = pos
	return
}


// The actual value must be within delta from the expected value.
func IsWithin(delta float64) Matcher {
	return func(actual_ interface{}, expected_ interface{}) (ok bool, pos os.Error, neg os.Error, err os.Error) {
		actual, err := toFloat64(actual_)
		if err != nil {
			return
		}
		expected, err := toFloat64(expected_)
		if err != nil {
			return
		}
		
		ok = math.Fabs(expected - actual) < delta
		pos = Errorf("Expected '%v' ± %v but was '%v'", expected, delta, actual)
		neg = Errorf("Did not expect '%v' ± %v but was '%v'", expected, delta, actual)
		return
	}
}

func toFloat64(actual interface{}) (result float64, err os.Error) {
	switch v := actual.(type) {
	case float:
		result = float64(v)
	case float32:
		result = float64(v)
	case float64:
		result = float64(v)
	default:
		err = Errorf("Expected a float, but was '%v' of type '%T'", actual, actual)
	}
	return
}


// The actual collection must contain the expected value.
func Contains(actual_ interface{}, expected interface{}) (ok bool, pos os.Error, neg os.Error, err os.Error) {
	actual, err := toArray(actual_)
	if err != nil {
		return
	}
	
	ok = arrayContains(actual, expected)
	pos = Errorf("Expected '%v' to be in '%v' but it was not", expected, actual)
	neg = Errorf("Did not expect '%v' to be in '%v' but it was", expected, actual)
	return
}

func arrayContains(haystack []interface{}, needle interface{}) bool {
	for i := 0; i < len(haystack); i++ {
		if areEqual(haystack[i], needle) {
			return true
		}
	}
	return false
}

func toArray(values interface{}) ([]interface{}, os.Error) {
	if it, ok := values.(iterable.Iterable); ok {
		return toArray(it.Iter())
	}
	
	result := new(vector.Vector)
	switch v := reflect.NewValue(values).(type) {
	
	case reflect.ArrayOrSliceValue:
		arr := v
		for i := 0; i < arr.Len(); i++ {
			obj :=  arr.Elem(i).Interface()
			result.Push(obj)
		}
		
	case *reflect.ChanValue:
		ch := v
		for {
			obj := ch.Recv().Interface()
			if ch.Closed() {
				break
			}
			result.Push(obj)
		}
		
	default:
		return nil, Errorf("Unknown type '%T', not iterable: %v", values, values)
	}
	return *result, nil
}


// The actual collection must contain all expected elements.
// The order of elements is not significant.
func ContainsAll(actual_ interface{}, expected_ interface{}) (ok bool, pos os.Error, neg os.Error, err os.Error) {
	actual, err := toArray(actual_)
	if err != nil {
		return
	}
	expected, err := toArray(expected_)
	if err != nil {
		return
	}
	
	containsAll := true
	for i := 0; i < len(expected); i++ {
		if !arrayContains(actual, expected[i]) {
			containsAll = false
			break
		}
	}
	
	ok = containsAll
	pos = Errorf("Expected all of '%v' to be in '%v' but they were not", expected, actual)
	neg = Errorf("Did not expect all of '%v' to be in '%v' but they were", expected, actual)
	return
}


// TODO: ContainsAny - The actual collection must contain at least one element from the given collection.
// TODO: ContainsExactly - The actual collection must contain exactly the same elements as in the given collection. The order of elements is not significant.
// TODO: ContainsInOrder - The actual collection must contain exactly the same elements as in the given collection, and they must be in the same order.
// TODO: ContainsInPartialOrder - The actual collection can hold other objects, but the objects which are common in both collections must be in the same order. The actual collection can also repeat some elements. For example [1, 2, 2, 3, 4] contains in partial order [1, 2, 3]. See Wikipedia <http://en.wikipedia.org/wiki/Partial_order> for further information.
