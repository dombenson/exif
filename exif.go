// Copyright (c) 2012-2015 José Carlos Nieto, https://menteslibres.net/xiam
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to
// permit persons to whom the Software is furnished to do so, subject to
// the following conditions:
//
// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
// LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
// OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
// WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

// Package exif provides bindings for libexif.
package exif

/*
#include <stdlib.h>
#include <libexif/exif-data.h>
#include <libexif/exif-loader.h>
#include "_cgo/types.h"

exif_value_t* pop_exif_value(exif_stack_t *);
void free_exif_value(exif_value_t* n);
exif_stack_t* exif_dump(ExifData *);
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"unsafe"
)

// Error messages.
var (
	ErrNoExifData      = errors.New(`No EXIF data found.`)
	ErrFoundExifInData = errors.New(`Found EXIF header. OK to call Parse.`)
)

const TagOrientation = 274

const TagLatitudeRef = 1
const TagLatitude = 2
const TagLongitudeRef = 3
const TagLongitude = 4
const TagAltitudeRef = 5
const TagAltitude = 6

const LatitudeRefNorth = "N"
const LatitudeRefSouth = "S"
const LongitudeRefEast = "E"
const LongitudeRefWest = "W"
const AltitudeRefAbove = 0
const AltitudeRefBelow = 1

const OrientationUnknown = 0
const OrientationTopLeft = 1
const OrientationTopRight = 2
const OrientationBottomRight = 3
const OrientationBottomLeft = 4
const OrientationLeftTop = 5
const OrientationRightTop = 6
const OrientationRightBottom = 7
const OrientationLeftBottom = 8

const exifFormatByte = 1
const exifFormatString = 2
const exifFormatShort = 3
const exifFormatLong = 4
const exifFormatFloat = 5

type Tag interface {
	Tag() int
	TextLabel() string
	TextValue() string
	setTag(int)
	setTextLabel(string)
	setTextValue(string)
}

type IntegerTag interface {
	Tag
	IntValue() int
}

type FloatTag interface {
	Tag
	FloatValue() float64
	Numerator() int
	Denominator() int
}

type basicTag struct {
	tag   int
	label string
	value string
}

type integerTag struct {
	basicTag
	intValue int
}

type floatTag struct {
	basicTag
	numerator   int
	denominator int
}

func (this *basicTag) Tag() int {
	return this.tag
}
func (this *basicTag) TextLabel() string {
	return this.label
}
func (this *basicTag) TextValue() string {
	return this.value
}

func (this *basicTag) setTag(val int) {
	this.tag = val
}
func (this *basicTag) setTextLabel(val string) {
	this.label = val
}
func (this *basicTag) setTextValue(val string) {
	this.value = val
}
func (this *integerTag) IntValue() int {
	return this.intValue
}
func (this *floatTag) Numerator() int {
	return this.numerator
}
func (this *floatTag) Denominator() int {
	return this.denominator
}

func (this *floatTag) FloatValue() float64 {
	return (float64(this.numerator) / float64(this.denominator))
}

// Data stores the EXIF tags of a file.
type Data struct {
	exifLoader *C.ExifLoader
	Tags       map[int]Tag
}

// New creates and returns a new exif.Data object.
func New() *Data {
	data := &Data{
		Tags: make(map[int]Tag),
	}
	return data
}

// Read attempts to read EXIF data from a file.
func Read(file string) (*Data, error) {
	data := New()
	if err := data.Open(file); err != nil {
		return nil, err
	}
	return data, nil
}

// Open opens a file path and loads its EXIF data.
func (d *Data) Open(file string) error {

	cfile := C.CString(file)
	defer C.free(unsafe.Pointer(cfile))

	exifData := C.exif_data_new_from_file(cfile)

	if exifData == nil {
		return ErrNoExifData
	}
	defer C.exif_data_unref(exifData)

	return d.parseExifData(exifData)
}

func (d *Data) parseExifData(exifData *C.ExifData) error {
	values := C.exif_dump(exifData)
	defer C.free(unsafe.Pointer(values))

	var byteOrder C.ExifByteOrder
	var haveByteOrder bool

	for {
		value := C.pop_exif_value(values)
		if value == nil {
			break
		} else {
			if !haveByteOrder {
				byteOrder = C.exif_data_get_byte_order((*value).rawValue.parent.parent)
				haveByteOrder = true
			}
			tagId := int(C.int((*value).rawValue.tag))
			tagFmt := C.int((*value).rawValue.format)
			var thisTag Tag
			if tagFmt == exifFormatByte {
				intTag := &integerTag{}
				thisTag = intTag
				intTag.intValue = int((*(*value).rawValue.data))
			} else if tagFmt == exifFormatShort {
				intTag := &integerTag{}
				thisTag = intTag
				intTag.intValue = int(C.exif_get_short((*value).rawValue.data, byteOrder))
			} else if tagFmt == exifFormatLong {
				intTag := &integerTag{}
				thisTag = intTag
				intTag.intValue = int(C.exif_get_long((*value).rawValue.data, byteOrder))
			} else if tagFmt == exifFormatFloat {
				intTag := &floatTag{}
				thisTag = intTag
				rational := C.exif_get_rational((*value).rawValue.data, byteOrder)
				intTag.numerator = int(rational.numerator)
				intTag.denominator = int(rational.denominator)
				numComponents := int((*value).rawValue.components)
				if numComponents > 1 {
					for i := 1; i < numComponents; i++ {
						rational = C.exif_get_rational_offset((*value).rawValue.data, byteOrder, C.int(i))
						intTag.numerator = 60*intTag.numerator*int(rational.denominator) + int(rational.numerator)*intTag.denominator
						intTag.denominator = intTag.denominator * int(rational.denominator) * 60
					}
				}
			} else {
				thisTag = &basicTag{}
			}
			thisTag.setTag(tagId)
			thisTag.setTextLabel(strings.Trim(C.GoString((*value).name), " "))
			thisTag.setTextValue(strings.Trim(C.GoString((*value).value), " "))
			d.Tags[thisTag.Tag()] = thisTag
		}
		C.free_exif_value(value)
	}

	return nil
}

// Write writes bytes to the exif loader. Sends ErrFoundExifInData error when
// enough bytes have been sent.
func (d *Data) Write(p []byte) (n int, err error) {
	if d.exifLoader == nil {
		d.exifLoader = C.exif_loader_new()
		runtime.SetFinalizer(d, (*Data).cleanup)
	}

	res := C.exif_loader_write(d.exifLoader, (*C.uchar)(unsafe.Pointer(&p[0])), C.uint(len(p)))

	if res == 1 {
		return len(p), nil
	}
	return len(p), ErrFoundExifInData
}

// Parse finalizes the data loader and sets the tags
func (d *Data) Parse() error {
	defer d.cleanup()

	exifData := C.exif_loader_get_data(d.exifLoader)
	if exifData == nil {
		return fmt.Errorf(ErrNoExifData.Error(), "")
	}

	defer func() {
		C.exif_data_unref(exifData)
	}()

	return d.parseExifData(exifData)
}

func (d *Data) cleanup() {
	if d.exifLoader != nil {
		C.exif_loader_unref(d.exifLoader)
		d.exifLoader = nil
	}
}
