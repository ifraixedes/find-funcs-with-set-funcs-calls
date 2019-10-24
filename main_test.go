package main

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFuncCalls(t *testing.T) {
	type returnVals struct {
		isError   bool
		funcCalls []funcCall
	}

	tcases := []struct {
		name     string
		in       string
		expected returnVals
	}{
		{
			name: "error: only package path",
			in:   "ioutil.ReadAll,net/http,strings.Compare",
			expected: returnVals{
				isError: true,
			},
		},
		{
			name: "error: bad formatted package path (ends with .)",
			in:   "ioutil.ReadAll.,strings.Compare",
			expected: returnVals{
				isError: true,
			},
		},
		{
			name: "error: bad formatted package path (ends with /)",
			in:   "ioutil.ReadAll,strings.Compare,storj.io/storj/pkg/storj/",
			expected: returnVals{
				isError: true,
			},
		},
		{
			name: "ok: standard packages",
			in:   "ioutil.ReadAll,strings.Compare,bytes.Buffer.Bytes",
			expected: returnVals{
				funcCalls: []funcCall{
					{
						pkg:      "ioutil",
						funcName: "ReadAll",
					},
					{
						pkg:      "strings",
						funcName: "Compare",
					},
					{
						pkg:      "bytes",
						receiver: "Buffer",
						funcName: "Bytes",
					},
				},
			},
		},
		{
			name: "ok: third party packages",
			in:   "storj.io/storj/pkg/storj.NewPieceKey,storj.io/storj/pkg/storj.IDVersion.GetIDVersion",
			expected: returnVals{
				funcCalls: []funcCall{
					{
						pkg:      "storj.io/storj/pkg/storj",
						funcName: "NewPieceKey",
					},
					{
						pkg:      "storj.io/storj/pkg/storj",
						receiver: "IDVersion",
						funcName: "GetIDVersion",
					},
				},
			},
		},
		{
			name: "ok: standard third party packages",
			in:   "storj.io/storj/pkg/storj.NewPieceKey,bytes.Buffer.Bytes,storj.io/storj/pkg/storj.IDVersion.GetIDVersion,strings.Compare",
			expected: returnVals{
				funcCalls: []funcCall{
					{
						pkg:      "storj.io/storj/pkg/storj",
						funcName: "NewPieceKey",
					},
					{
						pkg:      "bytes",
						receiver: "Buffer",
						funcName: "Bytes",
					},
					{
						pkg:      "storj.io/storj/pkg/storj",
						receiver: "IDVersion",
						funcName: "GetIDVersion",
					},
					{
						pkg:      "strings",
						funcName: "Compare",
					},
				},
			},
		},
		{
			name: "ok: spaces before and end any func call",
			in:   "storj.io/storj/pkg/storj.NewPieceKey, bytes.Buffer.Bytes,storj.io/storj/pkg/storj.IDVersion.GetIDVersion , strings.Compare",
			expected: returnVals{
				funcCalls: []funcCall{
					{
						pkg:      "storj.io/storj/pkg/storj",
						funcName: "NewPieceKey",
					},
					{
						pkg:      "bytes",
						receiver: "Buffer",
						funcName: "Bytes",
					},
					{
						pkg:      "storj.io/storj/pkg/storj",
						receiver: "IDVersion",
						funcName: "GetIDVersion",
					},
					{
						pkg:      "strings",
						funcName: "Compare",
					},
				},
			},
		},
	}

	for _, tc := range tcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			fcs, err := parseFuncCalls(tc.in)
			if tc.expected.isError {
				require.Error(t, err)
			}

			require.Equal(t, tc.expected.funcCalls, fcs)
		})
	}
}

func TestIntersect(t *testing.T) {
	type inparams struct {
		a []string
		b []string
	}
	tcases := []struct {
		name     string
		in       inparams
		expected []string
	}{
		{
			name: "2 lists are equal",
			in: inparams{
				a: []string{"abc", "bde", "xyz", "10"},
				b: []string{"abc", "bde", "xyz", "10"},
			},
			expected: []string{"10", "abc", "bde", "xyz"},
		},
		{
			name: "List A contains B",
			in: inparams{
				a: []string{"abc", "bde", "xyz", "10"},
				b: []string{"10", "bde", "abc"},
			},
			expected: []string{"10", "abc", "bde"},
		},
		{
			name: "List B contains A",
			in: inparams{
				a: []string{"bde", "xyz"},
				b: []string{"abc", "bde", "xyz", "10"},
			},
			expected: []string{"bde", "xyz"},
		},
		{
			name: "List A and B don't intersect",
			in: inparams{
				a: []string{"jkl", "mno", "55"},
				b: []string{"abc", "bde", "xyz", "10"},
			},
			expected: []string{},
		},
	}

	for _, tc := range tcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			intersection := intersect(tc.in.a, tc.in.b)
			require.Equal(t, tc.expected, intersection)
		})
	}
}

func TestFind(t *testing.T) {
	t.Run("finds some functions", func(t *testing.T) {
		cmdp, err := params([]string{
			"-funcs", "path/filepath.Join,strings.Compare,bytes.Buffer.Reset,github.com/ifraixedes/find-funcs-with-set-funcs-calls/testdata/testpkg.ExportedFunc,net/http/cookiejar.Jar.Cookies",
			"github.com/ifraixedes/find-funcs-with-set-funcs-calls/testdata/testpkg",
		})
		require.NoError(t, err)

		list, err := find(cmdp.pkgsPatterns, cmdp.funcCalls)
		require.NoError(t, err)
		require.Len(t, list, 1)

		sort.Slice(list[0].FuncNames, func(i, j int) bool {
			return list[0].FuncNames[i] < list[0].FuncNames[j]
		})

		expectedFuncs := []string{
			"unexportedFunc",
			"*unexportedType.ExportedMethod",
			"ExportedType.unexportedMethod",
		}
		sort.Slice(expectedFuncs, func(i, j int) bool {
			return expectedFuncs[i] < expectedFuncs[j]
		})

		assert.Equal(t, expectedFuncs, list[0].FuncNames)
	})

	t.Run("finds nothing", func(t *testing.T) {
		cmdp, err := params([]string{
			"-funcs", "path/filepath.Join,strings.Compare,bytes.Buffer.UnreadByte,github.com/ifraixedes/find-funcs-with-set-funcs-calls/testdata/testpkg.ExportedFunc,net/http/cookiejar.Jar.Cookies",
			"github.com/ifraixedes/find-funcs-with-set-funcs-calls/testdata/testpkg",
		})
		require.NoError(t, err)

		list, err := find(cmdp.pkgsPatterns, cmdp.funcCalls)
		require.NoError(t, err)
		require.Empty(t, list)
	})
}

func TestCreateSubsets(t *testing.T) {
	type inparams struct {
		fnCalls  []funcCall
		numElems uint
	}
	tcases := []struct {
		name     string
		in       inparams
		expected [][]funcCall
	}{
		{
			name: "numElems is 0",
			in: inparams{
				fnCalls: []funcCall{
					{pkg: "a", funcName: "f1"},
					{pkg: "a", receiver: "r1", funcName: "f1"},
					{pkg: "b", funcName: "f1"},
				},
				numElems: 0,
			},
			expected: [][]funcCall{
				[]funcCall{
					{pkg: "a", funcName: "f1"},
					{pkg: "a", receiver: "r1", funcName: "f1"},
					{pkg: "b", funcName: "f1"},
				},
			},
		},
		{
			name: "numElems is 1",
			in: inparams{
				fnCalls: []funcCall{
					{pkg: "a", funcName: "f1"},
					{pkg: "a", receiver: "r1", funcName: "f1"},
					{pkg: "b", funcName: "f1"},
				},
				numElems: 1,
			},
			expected: [][]funcCall{
				[]funcCall{{pkg: "a", funcName: "f1"}},
				[]funcCall{{pkg: "a", receiver: "r1", funcName: "f1"}},
				[]funcCall{{pkg: "b", funcName: "f1"}},
			},
		},
		{
			name: "numElems is equal length func calls",
			in: inparams{
				fnCalls: []funcCall{
					{pkg: "a", funcName: "f1"},
					{pkg: "a", receiver: "r1", funcName: "f1"},
					{pkg: "b", funcName: "f1"},
				},
				numElems: 3,
			},
			expected: [][]funcCall{
				[]funcCall{
					{pkg: "a", funcName: "f1"},
					{pkg: "a", receiver: "r1", funcName: "f1"},
					{pkg: "b", funcName: "f1"},
				},
			},
		},
		{
			name: "numElem is less than the length of func calls",
			in: inparams{
				fnCalls: []funcCall{
					{pkg: "a", funcName: "f"},
					{pkg: "b", receiver: "r", funcName: "f"},
					{pkg: "c", funcName: "f"},
					{pkg: "d", receiver: "r", funcName: "f"},
					{pkg: "e", funcName: "f"},
				},
				numElems: 2,
			},
			expected: [][]funcCall{
				[]funcCall{
					{pkg: "a", funcName: "f"},
					{pkg: "b", receiver: "r", funcName: "f"},
				},
				[]funcCall{
					{pkg: "a", funcName: "f"},
					{pkg: "c", funcName: "f"},
				},
				[]funcCall{
					{pkg: "a", funcName: "f"},
					{pkg: "d", receiver: "r", funcName: "f"},
				},
				[]funcCall{
					{pkg: "a", funcName: "f"},
					{pkg: "e", funcName: "f"},
				},
				[]funcCall{
					{pkg: "b", receiver: "r", funcName: "f"},
					{pkg: "c", funcName: "f"},
				},
				[]funcCall{
					{pkg: "b", receiver: "r", funcName: "f"},
					{pkg: "d", receiver: "r", funcName: "f"},
				},
				[]funcCall{
					{pkg: "b", receiver: "r", funcName: "f"},
					{pkg: "e", funcName: "f"},
				},
				[]funcCall{
					{pkg: "c", funcName: "f"},
					{pkg: "d", receiver: "r", funcName: "f"},
				},
				[]funcCall{
					{pkg: "c", funcName: "f"},
					{pkg: "e", funcName: "f"},
				},
				[]funcCall{
					{pkg: "d", receiver: "r", funcName: "f"},
					{pkg: "e", funcName: "f"},
				},
			},
		},
		{
			name: "numElem almost length of func calls",
			in: inparams{
				fnCalls: []funcCall{
					{pkg: "a"}, {pkg: "b"}, {pkg: "c"}, {pkg: "d"}, {pkg: "e"},
				},
				numElems: 4,
			},
			expected: [][]funcCall{
				[]funcCall{{pkg: "a"}, {pkg: "b"}, {pkg: "c"}, {pkg: "d"}},
				[]funcCall{{pkg: "a"}, {pkg: "b"}, {pkg: "c"}, {pkg: "e"}},
				[]funcCall{{pkg: "a"}, {pkg: "b"}, {pkg: "d"}, {pkg: "e"}},
				[]funcCall{{pkg: "a"}, {pkg: "c"}, {pkg: "d"}, {pkg: "e"}},
				[]funcCall{{pkg: "b"}, {pkg: "c"}, {pkg: "d"}, {pkg: "e"}},
			},
		},
	}

	for _, tc := range tcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			subsets := createSubsets(tc.in.fnCalls, tc.in.numElems)
			require.Len(t, subsets, len(tc.expected))
			require.Equal(t, tc.expected, subsets)
		})
	}
}

func TestMergeFuncsByFiles(t *testing.T) {
	type inparams struct {
		a []funcsByFile
		b []funcsByFile
	}
	tcases := []struct {
		name     string
		in       inparams
		expected []funcsByFile
	}{
		{
			name: "don't have same files",
			in: inparams{
				a: []funcsByFile{{Filename: "a.go", FuncNames: []string{"AFunc", "bFunc"}}},
				b: []funcsByFile{{Filename: "b.go", FuncNames: []string{"AFunc", "bFunc"}}},
			},
			expected: []funcsByFile{
				{Filename: "a.go", FuncNames: []string{"AFunc", "bFunc"}},
				{Filename: "b.go", FuncNames: []string{"AFunc", "bFunc"}},
			},
		},
		{
			name: "have some same files",
			in: inparams{
				a: []funcsByFile{
					{Filename: "a.go", FuncNames: []string{"AFunc", "bFunc"}},
					{Filename: "c.go", FuncNames: []string{"AFunc", "bFunc"}},
				},
				b: []funcsByFile{{Filename: "a.go", FuncNames: []string{"aFunc", "BFunc"}}},
			},
			expected: []funcsByFile{
				{Filename: "a.go", FuncNames: []string{"AFunc", "BFunc", "aFunc", "bFunc"}},
				{Filename: "c.go", FuncNames: []string{"AFunc", "bFunc"}},
			},
		},
		{
			name: "have some same files and same funcs",
			in: inparams{
				a: []funcsByFile{
					{Filename: "a.go", FuncNames: []string{"AFunc", "bFunc"}},
					{Filename: "c.go", FuncNames: []string{"AFunc", "bFunc"}},
				},
				b: []funcsByFile{{Filename: "c.go", FuncNames: []string{"AFunc", "BFunc"}}},
			},
			expected: []funcsByFile{
				{Filename: "a.go", FuncNames: []string{"AFunc", "bFunc"}},
				{Filename: "c.go", FuncNames: []string{"AFunc", "BFunc", "bFunc"}},
			},
		},
		{
			name: "totally equal",
			in: inparams{
				a: []funcsByFile{{Filename: "a.go", FuncNames: []string{"AFunc", "bFunc"}}},
				b: []funcsByFile{{Filename: "a.go", FuncNames: []string{"AFunc", "bFunc"}}},
			},
			expected: []funcsByFile{{Filename: "a.go", FuncNames: []string{"AFunc", "bFunc"}}},
		},
	}

	for _, tc := range tcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			merge := mergeFuncsByFiles(tc.in.a, tc.in.b)

			sort.Slice(merge, func(i, j int) bool {
				return merge[i].Filename < merge[j].Filename
			})

			sort.Slice(tc.expected, func(i, j int) bool {
				return tc.expected[i].Filename < tc.expected[j].Filename
			})

			require.Len(t, merge, len(tc.expected))
			require.Equal(t, tc.expected, merge)
		})
	}
}
