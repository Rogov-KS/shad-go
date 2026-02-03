//go:build !solution

package genericsum

import (
	"math/cmplx"
	"sort"
	"sync"

	"golang.org/x/exp/constraints"
)

func Min[T constraints.Ordered](x, y T) T {
	if x < y {
		return x
	}
	return y
}

func SortSlice[S ~[]T, T constraints.Ordered](s S) S {
	sort.Slice(s, func(i, j int) bool {
		return s[i] < s[j]
	})
	return s
}

func MapsEqual[K comparable, V comparable](a, b map[K]V) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func SliceContains[S ~[]T, T comparable](s S, v T) bool {
	for _, item := range s {
		if item == v {
			return true
		}
	}
	return false
}

func MergeChans[T any](chs ...<-chan T) <-chan T {
	out := make(chan T)
	var wg sync.WaitGroup
	wg.Add(len(chs))

	for _, ch := range chs {
		go func(c <-chan T) {
			defer wg.Done()
			for val := range c {
				out <- val
			}
		}(ch)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

func IsHermitianMatrix[T constraints.Integer | constraints.Float | constraints.Unsigned | constraints.Complex](m [][]T) bool {
	if len(m) == 0 {
		return true
	}

	for i := range m {
		if len(m[i]) != len(m) {
			return false
		}
		for j := range m[i] {
			// Check if we're dealing with complex numbers
			switch any(m[i][j]).(type) {
			case complex64, complex128:
				// Convert both values to complex128 for comparison
				var valI128, valJ128 complex128
				switch v := any(m[i][j]).(type) {
				case complex64:
					valI128 = complex128(v)
				case complex128:
					valI128 = v
				}
				switch v := any(m[j][i]).(type) {
				case complex64:
					valJ128 = complex128(v)
				case complex128:
					valJ128 = v
				default:
					return false
				}
				if valI128 != cmplx.Conj(valJ128) {
					return false
				}
			default:
				// Handle real numeric types
				if m[i][j] != m[j][i] {
					return false
				}
			}
		}
	}
	return true
}
