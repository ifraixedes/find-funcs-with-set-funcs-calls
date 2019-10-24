package testpkg

import (
	"bytes"
	"fmt"
	"net/http/cookiejar"
	"net/url"
	"path/filepath"
	strs "strings"
)

func unexportedFunc() {
	both := filepath.Join("x", "y")
	buf := bytes.NewBufferString(both)
	if buf.Len() > 0 {
		buf.Reset()
	}

	strs.Compare("wow", "lol")
	ExportedFunc()

	jar, _ := cookiejar.New(nil)
	jar.Cookies(&url.URL{})
}

// ExportedFunc ...
func ExportedFunc() {
	fmt.Println("hello")
}

// CallFuncByCalledFuncs ...
func CallFuncByCalledFuncs(s string) {
	sb := bufferFromString(s)

	if s, ok := stringIfAreEqual(sb, bytes.NewBuffer([]byte("some"))); ok {
		printString(s)
	}
}

func stringIfAreEqual(b1 *bytes.Buffer, b2 *bytes.Buffer) (_ string, ok bool) {
	if bufferLen(*b1) != bufferLen(*b2) {
		return "", false
	}

	if strs.Compare(b1.String(), b2.String()) == 0 {
		return b1.String(), true
	}

	return "", false
}

func bufferFromString(s string) *bytes.Buffer {
	return bytes.NewBufferString(s)
}

func bufferLen(b bytes.Buffer) int {
	return b.Len()
}

func printString(s string) {
	fmt.Println(s)
}

type unexportedType string

func (*unexportedType) unexportedMethod() {
}

func (*unexportedType) ExportedMethod() {
	jar, _ := cookiejar.New(nil)
	jar.Cookies(&url.URL{})

	_ = filepath.Join("x", "y", "-")
	expt := ExportedType{}
	expt.content.sc.b.Reset()

	strs.Compare("wow", "lol")
	ExportedFunc()
}

type content struct {
	sc subcontent
}

type subcontent struct {
	b bytes.Buffer
}

// ExportedType ...
type ExportedType struct {
	content
}

func (ExportedType) unexportedMethod() {
	expt := &ExportedType{}
	expt.content.sc.b.Reset()

	jar, _ := cookiejar.New(nil)
	jar.Cookies(&url.URL{})

	_ = filepath.Join("x", "y", "-")

	strs.Compare("wow", "lol")
	ExportedFunc()
}

// ExportedMethod ...
func (ExportedType) ExportedMethod() {
	both := filepath.Join("x", "y")
	buf := bytes.NewBufferString(both)
	if buf.Len() > 0 {
		buf.Reset()
	}

	strs.Compare("wow", "lol")
	ExportedFunc()
}
