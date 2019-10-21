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
