package util

import (
	"reflect"
	"slices"

	"golang.org/x/exp/constraints"
)

func CopyMap[T1 comparable, T2 any](m map[T1](T2)) map[T1](T2) {
	cp := map[T1](T2){}
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

func CopySlice[T any](src []T) []T {
	dst := make([]T, len(src))
	copy(dst, src)
	return dst
}

func Filter[T any](ss []T, test func(T) bool) (ret []T) {
	for _, s := range ss {
		if test(s) {
			ret = append(ret, s)
		}
	}
	return
}

func FilterNot[T any](ss []T, test func(T) bool) (ret []T) {
	for _, s := range ss {
		if !test(s) {
			ret = append(ret, s)
		}
	}
	return
}

func FindInSlice[T any](slice []T, checker func(T) bool) *T {
	index := slices.IndexFunc(slice, checker)
	if index == -1 {
		return nil
	}
	return &slice[index]
}

func Map[T1 any, T2 any](ss []T1, mapper func(T1) T2) (ret []T2) {
	for _, s := range ss {
		ret = append(ret, mapper(s))
	}
	return
}

func MapMaxElementKey[TK comparable, TV constraints.Ordered](m map[TK](TV)) TK {
	var result TK
	var resultValue TV
	i := int64(0)
	for key, value := range m {
		if i == 0 {
			result = key
			resultValue = value
		} else if value > resultValue {
			result = key
			resultValue = value
		}
		i++
	}
	return result
}

func UniqueSlice[T comparable](slice []T) []T {
	keys := map[T]bool{}
	list := []T{}
	for _, entry := range slice {
		if !keys[entry] {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

// Return de-duplicated slice that every member has unique key.
func UniqueSliceFn[TS any, TK comparable](slice []TS, keyFunc func(TS) TK) []TS {
	keys := map[TK]bool{}
	list := []TS{}
	for _, entry := range slice {
		key := keyFunc(entry)
		if !keys[key] {
			keys[key] = true
			list = append(list, entry)
		}
	}
	return list
}

func MapKeys[T constraints.Ordered, TV any](input map[T]TV) []T {
	keys := make([]T, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// From https://stackoverflow.com/questions/23589564/function-for-converting-a-struct-to-map-in-golang .
func StructToMap(val interface{}, ignoreNoTagFields bool, ignoreEmptyFields bool) map[string]interface{} {
	//The name of the tag you will use for fields of struct
	const tagTitle = "yaml"

	var data map[string]interface{} = make(map[string]interface{})
	varType := reflect.TypeOf(val)
	if varType.Kind() != reflect.Struct {
		// Provided value is not an interface, do what you will with that here
		panic("Not a struct")
	}

	value := reflect.ValueOf(val)
	for i := 0; i < varType.NumField(); i++ {
		if !value.Field(i).CanInterface() {
			//Skip unexported fields
			continue
		}
		tag, ok := varType.Field(i).Tag.Lookup(tagTitle)
		var fieldName string
		if ok && len(tag) > 0 {
			fieldName = tag
		} else if ignoreNoTagFields {
			continue
		} else {
			fieldName = varType.Field(i).Name
		}
		fieldKind := varType.Field(i).Type.Kind()
		fieldValue := value.Field(i)
		if fieldKind != reflect.Struct {
			if ignoreEmptyFields {
				if fieldKind == reflect.String && fieldValue.String() == "" {
					continue
				}
				if fieldKind == reflect.Int64 && fieldValue.Int() == 0 {
					continue
				}
				if fieldKind == reflect.Float64 && fieldValue.Float() == 0 {
					continue
				}
				if fieldKind == reflect.Bool && !fieldValue.Bool() {
					continue
				}
				if fieldKind == reflect.Slice && fieldValue.Pointer() == 0 {
					continue
				}
				if fieldKind == reflect.Pointer && fieldValue.Pointer() == 0 {
					continue
				}
			}
			data[fieldName] = fieldValue.Interface()
		} else {
			data[fieldName] = StructToMap(fieldValue.Interface(), ignoreNoTagFields, ignoreEmptyFields)
		}
	}

	return data
}
