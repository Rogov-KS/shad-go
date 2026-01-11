//go:build !solution

package testequal

import (
	"fmt"
	"reflect"
)

func IsEqual(expected, actual interface{}) bool {
	expectedType := reflect.TypeOf(expected)
	actualType := reflect.TypeOf(actual)

	if expectedType != actualType {
		return false
	}

	kind := actualType.Kind()

	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.String, reflect.Bool:

		return expected == actual

	case reflect.Slice:
		expectedValue := reflect.ValueOf(expected)
		actualValue := reflect.ValueOf(actual)
		// Различаем nil и пустой слайс
		if expectedValue.IsNil() != actualValue.IsNil() {
			return false
		}
		if expectedValue.Len() != actualValue.Len() {
			return false
		}
		for i := 0; i < expectedValue.Len(); i++ {
			if !IsEqual(expectedValue.Index(i).Interface(), actualValue.Index(i).Interface()) {
				return false
			}
		}
		return true

	case reflect.Map:
		expectedValue := reflect.ValueOf(expected)
		actualValue := reflect.ValueOf(actual)
		// Различаем nil и пустую мапу
		if expectedValue.IsNil() != actualValue.IsNil() {
			return false
		}
		if expectedValue.Len() != actualValue.Len() {
			return false
		}
		for _, key := range expectedValue.MapKeys() {
			exRes := expectedValue.MapIndex(key)
			acRes := actualValue.MapIndex(key)
			if !acRes.IsValid() || !IsEqual(exRes.Interface(), acRes.Interface()) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func formatMessage(msgAndArgs ...interface{}) string {
	if len(msgAndArgs) == 0 {
		return ""
	}
	if len(msgAndArgs) == 1 {
		return fmt.Sprintf("%v", msgAndArgs[0])
	}
	format, ok := msgAndArgs[0].(string)
	if !ok {
		return fmt.Sprintf("%v", msgAndArgs)
	}
	return fmt.Sprintf(format, msgAndArgs[1:]...)
}

func AssertEqual(t T, expected, actual interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()

	isEqual := IsEqual(expected, actual)

	if !isEqual {
		msg := formatMessage(msgAndArgs...)
		t.Errorf("==== Run Test\n---- Fail Test:\n\texpected: %v\n\tactual  : %v\n\tmessage : %s\n\n", expected, actual, msg)
	}
	return isEqual
}

// AssertNotEqual checks that expected and actual are not equal.
//
// Marks caller function as having failed but continues execution.
//
// Returns true iff arguments are not equal.
func AssertNotEqual(t T, expected, actual interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()

	isEqual := IsEqual(expected, actual)

	if isEqual {
		msg := formatMessage(msgAndArgs...)
		t.Errorf("==== Run Test\n---- Fail Test:\n\texpected: %v\n\tactual  : %v\n\tmessage : %s\n\n", expected, actual, msg)
	}
	return !isEqual
}

// RequireEqual does the same as AssertEqual but fails caller test immediately.
func RequireEqual(t T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper()

	isEqual := IsEqual(expected, actual)

	if !isEqual {
		msg := formatMessage(msgAndArgs...)
		t.Errorf("==== Run Test\n---- Fail Test:\n\texpected: %v\n\tactual  : %v\n\tmessage : %s\n\n", expected, actual, msg)
		t.FailNow()
	}
}

// RequireNotEqual does the same as AssertNotEqual but fails caller test immediately.
func RequireNotEqual(t T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper()

	isEqual := IsEqual(expected, actual)

	if isEqual {
		msg := formatMessage(msgAndArgs...)
		t.Errorf("==== Run Test\n---- Fail Test:\n\texpected: %v\n\tactual  : %v\n\tmessage : %s\n\n", expected, actual, msg)
		t.FailNow()
	}
}
