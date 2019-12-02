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


package senc

import (
	"io"
	"github.com/mad-day/lz4"
	"github.com/vmihailenco/msgpack"
	"github.com/klauspost/compress/fse"
	"github.com/klauspost/compress/huff0"
	"fmt"
)

const (
	t_raw = iota
	t_fse
	t_huff0
)

type packet struct {
	srclen uint
	ct,lt byte
	cd,ld []byte
}
func (p *packet) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(p.ct,p.lt,p.cd,p.ld)
}
func (p *packet) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.DecodeMulti(&p.ct,&p.lt,&p.cd,&p.ld)
}

type entropy struct {
	fse fse.Scratch
	hf0 huff0.Scratch
}
func (c *entropy) decode(t byte, d []byte) ([]byte,error) {
	switch t {
	case t_raw: return d,nil
	case t_fse: return fse.Decompress(d,&c.fse)
	case t_huff0: return c.hf0.Decompress1X(d)
	}
	return nil,fmt.Errorf("unknown format (%x)",t)
}
func (c *entropy) encode(in []byte) (t byte, d []byte, err error) {
	t = t_fse
	d,err = fse.Compress(in,&c.fse)
	if err==fse.ErrUseRLE { goto use_rle }
	if err==fse.ErrIncompressible { goto dont_compress }
	
	//if d2,_,e2 := huff0.Compress1X(in,&c.hf0); e2==nil {
	//	if len(d2)<len(d) { return t_huff0,d2,nil }
	//}
	return
	
	/*
	retry_compress:
	t = t_huff0
	d,_,err = huff0.Compress1X(in,&c.hf0)
	if err==huff0.ErrUseRLE { goto use_rle }
	if err!=nil { goto dont_compress }
	
	return
	*/
	dont_compress:
	return t_raw,in,nil
	
	use_rle:
	// TODO: implement RLE
	return t_raw,in,nil
}
func (c *entropy) reset() {
	// TODO: more fine-grained cleanup.
	c.hf0 = huff0.Scratch{}
	c.hf0.Reuse = huff0.ReusePolicyNone
}

type codec struct {
	c,l entropy
	lzblock []byte
	dblock  []byte
	rcd,rld []byte
}
func (c *codec) decode(pkt *packet) ([]byte,error) {
	cd,err1 := c.c.decode(pkt.ct,pkt.cd)
	ld,err2 := c.l.decode(pkt.lt,pkt.ld)
	if err1!=nil { return nil,err1 }
	if err2!=nil { return nil,err2 }
	
	c.lzblock = c.lzblock[:0]
	err1 = lz4.MergeCompressedBlock(cd,ld,&c.lzblock)
	fmt.Println("err1 =",err1)
	if err1!=nil { return nil,err1 }
	
	if len(c.dblock)<int(pkt.srclen) {
		c.dblock = make([]byte,int(pkt.srclen)+100)
	}
	lng,err := lz4.UncompressBlock(c.lzblock,c.dblock,0)
	if err!=nil { return nil,err }
	return c.dblock[:lng],nil
}
func (c *codec) encode(d []byte,pkt *packet) error {
	
	if compl := lz4.CompressBlockBound(len(d)); cap(c.lzblock)<compl {
		c.lzblock = make([]byte,compl)
	} else {
		c.lzblock = c.lzblock[:compl]
	}
	
	sz,e := lz4.CompressBlock(d,c.lzblock,0)
	if e!=nil { return e }
	
	c.rcd = c.rcd[:0]
	c.rld = c.rld[:0]
	fmt.Printf("c.lzblock[:sz]\n%q\n",c.lzblock[:sz])
	e = lz4.SplitCompressedBlock(c.lzblock[:sz],&c.rcd,&c.rld)
	fmt.Println(sz,len(c.rcd)+len(c.rld))
	if e!=nil { return e }
	fmt.Println("lz4.MergeCompressedBlock(c.rcd,c.rld,new([]byte))",lz4.MergeCompressedBlock(c.rcd,c.rld,new([]byte)))
	
	ct,cd,ce := c.c.encode(c.rcd)
	lt,ld,le := c.l.encode(c.rld)
	
	if ce!=nil { return ce }
	if le!=nil { return le }
	
	*pkt = packet{srclen:uint(len(d)),ct:ct,lt:lt,cd:cd,ld:ld}
	return nil
}
func (c *codec) encodeHC(d []byte,pkt *packet) error {
	
	if compl := lz4.CompressBlockBound(len(d)); cap(c.lzblock)<compl {
		c.lzblock = make([]byte,compl)
	} else {
		c.lzblock = c.lzblock[:compl]
	}
	
	sz,e := lz4.CompressBlockHC(d,c.lzblock,0)
	if e!=nil { return e }
	
	c.rcd = c.rcd[:0]
	c.rld = c.rld[:0]
	e = lz4.SplitCompressedBlock(c.lzblock[:sz],&c.rcd,&c.rld)
	if e!=nil { return e }
	fmt.Println("lz4.MergeCompressedBlock(c.rcd,c.rld,new([]byte))",lz4.MergeCompressedBlock(c.rcd,c.rld,new([]byte)))
	
	ct,cd,ce := c.c.encode(c.rcd)
	lt,ld,le := c.l.encode(c.rld)
	
	if ce!=nil { return ce }
	if le!=nil { return le }
	
	*pkt = packet{uint(len(d)),ct,lt,cd,ld}
	return nil
}
func (c *codec) reset() {
	c.c.reset()
	c.l.reset()
}

type WriterConfig struct {
	UseHC bool
}

type Writer struct {
	e *msgpack.Encoder
	c codec
	p packet
	
	WriterConfig
}
func (xw *Writer) Reset(w io.Writer) {
	xw.e = msgpack.NewEncoder(w)
	xw.c.reset()
}
func (xw *Writer) Write(b []byte) (int,error) {
	var err error
	if xw.UseHC {
		err = xw.c.encodeHC(b,&xw.p)
	} else {
		err = xw.c.encode(b,&xw.p)
	}
	if err!=nil { return 0,err }
	err = xw.e.Encode(&xw.p)
	if err!=nil { return 0,err }
	return len(b),nil
}
func NewWriter(w io.Writer) *Writer {
	xw := new(Writer)
	xw.Reset(w)
	return xw
}

type Reader struct {
	d *msgpack.Decoder
	c codec
	p packet
	rest []byte
	err error
}
func (xr *Reader) Reset(r io.Reader) {
	if xr.d==nil {
		xr.d = msgpack.NewDecoder(r)
	} else {
		xr.d.Reset(r)
	}
	xr.c.reset()
}
func (xr *Reader) iread(b []byte,c *int) error {
	if len(xr.rest)>0 {
		i := copy(b,xr.rest)
		xr.rest = xr.rest[i:]
		b = b[i:]
		*c += i
	}
	for len(b)>0 {
		if xr.err!=nil { return xr.err }
		xr.err = xr.d.Decode(&xr.p)
		if xr.err!=nil { return xr.err }
		xr.rest,xr.err = xr.c.decode(&xr.p)
		if xr.err!=nil { return xr.err }
		
		i := copy(b,xr.rest)
		xr.rest = xr.rest[i:]
		b = b[i:]
		*c += i
	}
	return nil
}
func (xr *Reader) Read(b []byte) (c int,e error) {
	e = xr.iread(b,&c)
	return
}
func NewReader(w io.Reader) *Reader {
	xw := new(Reader)
	xw.Reset(w)
	return xw
}

