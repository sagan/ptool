package utils

import (
	"golang.org/x/exp/constraints"
	"golang.org/x/exp/slices"
)

func CopyMap[T1 comparable, T2 any](m map[T1](T2)) map[T1](T2) {
	cp := make(map[T1](T2))
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
	keys := make(map[T]bool)
	list := []T{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

// return de-duplicated slice that every member has unique key
func UniqueSliceFn[TS any, TK comparable](slice []TS, keyFunc func(TS) TK) []TS {
	keys := make(map[TK]bool)
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
