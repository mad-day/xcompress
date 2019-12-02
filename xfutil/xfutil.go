/*
Copyright (c) 2019 Simon Schmidt

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/


// A helper package to make the xflate package more useful.
// See https://github.com/dsnet/compress or
// https://godoc.org/github.com/dsnet/compress/xflate for more information.
package xfutil

import (
	"io"
)

type Reader struct {
	base  io.ReadSeeker
	state int64
}
func NewReader(r io.ReadSeeker) *Reader {
	xr := new(Reader)
	xr.Reset(r)
	return xr
}
func (xr *Reader) Reset(r io.ReadSeeker) {
	xr.state = -1
	xr.base = r
}
func (xr *Reader) Seek(offset int64, whence int) (int64, error) {
	if xr.state!=-1 {
		if whence==io.SeekCurrent {
			offset += xr.state
			whence = io.SeekStart
		}
		xr.state = -1
	}
	return xr.base.Seek(offset,whence)
}
func (xr *Reader) Read(p []byte) (n int, err error) {
	if xr.state!=-1 {
		_,err = xr.base.Seek(xr.state,io.SeekStart)
		if err!=nil { return }
		xr.state = -1
	}
	return xr.base.Read(p)
}
func (xr *Reader) ReadAt(p []byte, off int64) (n int, err error) {
	var u int64
	if xr.state==-1 {
		u,err = xr.base.Seek(0,io.SeekCurrent)
		if err!=nil { return }
		xr.state = u
	}
	xr.base.Seek(off,io.SeekStart)
	n,err = io.ReadFull(xr.base, p)
	if err==io.ErrUnexpectedEOF { err = io.EOF }
	return
}
var _ io.Seeker   = (*Reader)(nil)
var _ io.Reader   = (*Reader)(nil)
var _ io.ReaderAt = (*Reader)(nil)

type ReaderAt struct {
	Inner io.ReadSeeker
}
func (xr *ReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	_,err = xr.Inner.Seek(off,io.SeekStart)
	if err!=nil { return }
	n,err = io.ReadFull(xr.Inner, p)
	if err==io.ErrUnexpectedEOF { err = io.EOF }
	return
}
var _ io.ReaderAt = (*ReaderAt)(nil)


