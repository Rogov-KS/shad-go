//go:build !solution

package reversemap

import "reflect"

var supportedKinds = map[reflect.Kind]bool{
	reflect.Int:        true,
	reflect.Uint:       true,
	reflect.String:     true,
	reflect.Complex64:  true,
	reflect.Complex128: true,
}

func isSupportedKind(kind reflect.Kind) bool {
	return supportedKinds[kind]
}

func ReverseMap(forward interface{}) interface{} {
	if reflect.TypeOf(forward).Kind() != reflect.Map {
		panic("forward is not a map")
	}
	keysType := reflect.TypeOf(forward).Key()
	valuesType := reflect.TypeOf(forward).Elem()
	if !isSupportedKind(keysType.Kind()) {
		panic("keys type is not supported")
	}
	if !isSupportedKind(valuesType.Kind()) {
		panic("values type is not supported")
	}
	forwardValue := reflect.ValueOf(forward)
	backwardType := reflect.MapOf(valuesType, keysType)
	backward := reflect.MakeMap(backwardType)

	for iter := forwardValue.MapRange(); iter.Next(); {
		key := iter.Key()
		value := iter.Value()
		backward.SetMapIndex(value, key)
	}
	return backward.Interface()
}
