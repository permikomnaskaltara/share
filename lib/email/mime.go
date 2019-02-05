// Copyright 2019, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package email

import (
	"bytes"
	"errors"
	"strings"

	libio "github.com/shuLhan/share/lib/io"
)

//
// MIME represent part of message body with their header and content.
//
type MIME struct {
	Header  Header
	Content []byte
}

//
// ParseBodyPart parse one body part using boundary and return the rest of
// body.
//
func ParseBodyPart(raw, boundary []byte) (mime *MIME, rest []byte, err error) {
	if len(raw) == 0 {
		return nil, raw, nil
	}
	if len(boundary) == 0 {
		return nil, raw, errors.New("ParseBodyPart: boundary parameter is empty")
	}

	r := &libio.Reader{}
	r.InitBytes(raw)
	var (
		line   []byte
		minlen = len(boundary) + 2
	)

	// find boundary ...
	r.SkipSpace()
	line = r.ReadLine()
	if len(line) == 0 {
		rest = r.Rest()
		return nil, rest, nil
	}
	if len(line) < minlen {
		return nil, raw, errors.New("ParseBodyPart: missing boundary line")
	}
	if line[len(line)-2] != '\r' {
		return nil, raw, errors.New("ParseBodyPart: invalid boundary line: missing CR")
	}
	if !bytes.Equal(line[:2], boundSeps) {
		return nil, raw, errors.New("ParseBodyPart: invalid boundary line: missing '--'")
	}
	if !bytes.Equal(line[2:minlen], boundary) {
		return nil, raw, errors.New("ParseBodyPart: boundary mismatch")
	}
	if bytes.Equal(line[minlen:len(line)-2], boundSeps) {
		// End of body.
		return nil, r.Rest(), nil
	}

	mime = &MIME{}
	rest, err = mime.Header.Unpack(r.Rest())
	if err != nil {
		return nil, raw, err
	}

	r.InitBytes(rest)

	for {
		line = r.ReadLine()
		if len(line) == 0 {
			break
		}
		if len(line) < minlen {
			mime.Content = append(mime.Content, line...)
			continue
		}
		if line[len(line)-2] != '\r' {
			mime.Content = append(mime.Content, line...)
			continue
		}
		if !bytes.Equal(line[:2], boundSeps) {
			mime.Content = append(mime.Content, line...)
			continue
		}
		if !bytes.Equal(line[2:minlen], boundary) {
			mime.Content = append(mime.Content, line...)
			continue
		}
		r.UnreadN(len(line))
		break
	}

	rest = r.Rest()

	return mime, rest, err
}

//
// String return string representation of MIME object.
//
func (mime *MIME) String() string {
	var sb strings.Builder

	sb.WriteString(mime.Header.String())
	sb.WriteString("\r\n")
	sb.Write(mime.Content)

	return sb.String()
}
