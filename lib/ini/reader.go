// Copyright 2018, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ini

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"unicode"

	"github.com/shuLhan/share/lib/debug"
)

const (
	tokBackslash   = '\\'
	tokBackspace   = '\b'
	tokDot         = '.'
	tokDoubleQuote = '"'
	tokEqual       = '='
	tokHash        = '#'
	tokHyphen      = '-'
	tokNewLine     = '\n'
	tokPercent     = '%'
	tokSecEnd      = ']'
	tokSecStart    = '['
	tokSemiColon   = ';'
	tokSpace       = ' '
	tokTab         = '\t'
	tokUnderscore  = '_'
)

var (
	errBadConfig      = errors.New("bad config line %d at %s")
	errVarNoSection   = "variable without section, line %d at %s"
	errVarNameInvalid = errors.New("invalid variable name, line %d at %s")
	errValueInvalid   = errors.New("invalid value, line %d at %s")
)

//
// reader define the INI file reader.
//
type reader struct {
	br         *bytes.Reader
	b          byte
	r          rune
	lineNum    int
	filename   string
	_var       *Variable
	sec        *Section
	buf        bytes.Buffer
	bufComment bytes.Buffer
	bufFormat  bytes.Buffer
	bufSpaces  bytes.Buffer
}

//
// newReader create, initialize, and return new reader.
//
func newReader() (r *reader) {
	r = &reader{
		br: bytes.NewReader(nil),
	}
	r.reset(nil)

	return
}

//
// reset all reader attributes, excluding filename.
//
func (reader *reader) reset(src []byte) {
	reader.br.Reset(src)
	reader.b = 0
	reader.r = 0
	reader.lineNum = 0
	reader._var = &Variable{
		mode: varModeEmpty,
	}
	reader.sec = &Section{
		mode: varModeEmpty,
	}
	reader.buf.Reset()
	reader.bufComment.Reset()
	reader.bufFormat.Reset()
	reader.bufSpaces.Reset()
}

//
// parseFile will open, read, and parse INI file `filename` and return an
// instance of Ini.
//
// On failure, it return nil and error.
//
func (reader *reader) parseFile(filename string) (in *Ini, err error) {
	src, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}

	reader.filename = filename

	in, err = reader.Parse(src)

	return
}

//
// Parse will parse INI config from slice of bytes `src` into `in`.
//
func (reader *reader) Parse(src []byte) (in *Ini, err error) {
	in = &Ini{}
	reader.reset(src)

	for {
		reader.lineNum++

		err = reader.parse()
		if err != nil {
			if err != io.EOF {
				err = fmt.Errorf(err.Error(), reader.lineNum,
					reader.filename)
				return nil, err
			}
			break
		}

		if debug.Value >= 1 {
			fmt.Print(reader._var)
		}

		reader._var.lineNum = reader.lineNum

		if reader._var.mode&varModeSingle == varModeSingle ||
			reader._var.mode&varModeValue == varModeValue ||
			reader._var.mode&varModeMulti == varModeMulti {
			if reader.sec.mode == varModeEmpty {
				err = fmt.Errorf(errVarNoSection,
					reader.lineNum,
					reader.filename)
				return nil, err
			}
		}

		if reader._var.mode&varModeSection == varModeSection ||
			reader._var.mode&varModeSubsection == varModeSubsection {

			in.AddSection(reader.sec)

			reader.sec = &Section{
				mode:    reader._var.mode,
				LineNum: reader._var.lineNum,
				Name:    reader._var.secName,
				Sub:     reader._var.subName,
				format:  reader._var.format,
				others:  reader._var.others,
			}

			reader._var = &Variable{
				mode: varModeEmpty,
			}
			continue
		}

		reader.sec.add(reader._var)

		reader._var = &Variable{
			mode: varModeEmpty,
		}
	}

	if reader._var.mode != varModeEmpty {
		if debug.Value >= 1 {
			fmt.Println(reader._var)
		}

		reader.sec.add(reader._var)
	}

	in.AddSection(reader.sec)

	reader._var = nil
	reader.sec = nil

	return in, nil
}

func (reader *reader) parse() (err error) {
	reader.bufFormat.Reset()

	for {
		reader.b, err = reader.br.ReadByte()
		if err != nil {
			return err
		}
		if reader.b == tokNewLine {
			reader.bufFormat.WriteByte(reader.b)
			reader._var.format = reader.bufFormat.String()
			break
		}
		if reader.b == tokSpace || reader.b == tokTab {
			reader.bufFormat.WriteByte(reader.b)
			continue
		}
		if reader.b == tokHash || reader.b == tokSemiColon {
			_ = reader.br.UnreadByte()

			return reader.parseComment()
		}
		if reader.b == tokSecStart {
			_ = reader.br.UnreadByte()
			return reader.parseSectionHeader()
		}
		_ = reader.br.UnreadByte()
		return reader.parseVariable()
	}

	return nil
}

func (reader *reader) parseComment() (err error) {
	reader.bufComment.Reset()

	reader._var.mode |= varModeComment

	reader.bufFormat.Write([]byte{'%', 's'})

	for {
		reader.b, err = reader.br.ReadByte()
		if err != nil {
			break
		}
		if reader.b == tokNewLine {
			reader.bufFormat.WriteByte(reader.b)
			break
		}
		_ = reader.bufComment.WriteByte(reader.b)
	}

	reader._var.format = reader.bufFormat.String()
	reader._var.others = reader.bufComment.String()

	return
}

func (reader *reader) parseSectionHeader() (err error) {
	reader.buf.Reset()

	reader.b, err = reader.br.ReadByte()
	if err != nil {
		return errBadConfig
	}

	if reader.b != tokSecStart {
		return errBadConfig
	}

	reader.bufFormat.WriteByte(tokSecStart)
	reader._var.mode = varModeSection

	reader.r, _, err = reader.br.ReadRune()
	if err != nil {
		return errBadConfig
	}

	if !unicode.IsLetter(reader.r) {
		return errBadConfig
	}

	reader.bufFormat.Write([]byte{'%', 's'})
	reader.buf.WriteRune(reader.r)

	for {
		reader.r, _, err = reader.br.ReadRune()
		if err != nil {
			return errBadConfig
		}
		if reader.r == tokSpace || reader.r == tokTab {
			break
		}
		if reader.r == tokSecEnd {
			reader.bufFormat.WriteRune(reader.r)

			reader._var.secName = reader.buf.String()

			return reader.parsePossibleComment()
		}
		if unicode.IsLetter(reader.r) || unicode.IsDigit(reader.r) || reader.r == tokHyphen || reader.r == tokDot {
			reader.buf.WriteRune(reader.r)
			continue
		}

		return errBadConfig
	}

	reader.bufFormat.WriteRune(reader.r)
	reader._var.secName = reader.buf.String()

	return reader.parseSubsection()
}

//
// (0) Skip white-spaces
//
func (reader *reader) parseSubsection() (err error) {
	reader.buf.Reset()

	reader._var.mode |= varModeSubsection

	// (0)
	for {
		reader.b, err = reader.br.ReadByte()
		if err != nil {
			return errBadConfig
		}
		if reader.b == tokSpace || reader.b == tokTab {
			reader.bufFormat.WriteByte(reader.b)
			continue
		}
		if reader.b != tokDoubleQuote {
			return errBadConfig
		}
		break
	}

	reader.bufFormat.WriteByte(reader.b) // == tokDoubleQuote
	reader.bufFormat.Write([]byte{'%', 's'})

	var esc bool
	var end bool

	for {
		reader.b, err = reader.br.ReadByte()
		if err != nil {
			return errBadConfig
		}
		if end {
			if reader.b == tokSecEnd {
				reader.bufFormat.WriteByte(reader.b)
				break
			}
			return errBadConfig
		}
		if esc {
			reader.buf.WriteByte(reader.b)
			esc = false
			continue
		}
		if reader.b == tokBackslash {
			esc = true
			continue
		}
		if reader.b == tokDoubleQuote {
			reader.bufFormat.WriteByte(reader.b)
			end = true
			continue
		}
		reader.buf.WriteByte(reader.b)
	}

	reader._var.subName = reader.buf.String()

	return reader.parsePossibleComment()
}

//
// parsePossibleComment will check only for whitespace and comment start
// character.
//
func (reader *reader) parsePossibleComment() (err error) {
	for {
		reader.b, err = reader.br.ReadByte()
		if err != nil {
			break
		}
		if reader.b == tokNewLine {
			reader.bufFormat.WriteByte(reader.b)
			break
		}
		if reader.b == tokSpace || reader.b == tokTab {
			reader.bufFormat.WriteByte(reader.b)
			continue
		}
		if reader.b == tokHash || reader.b == tokSemiColon {
			_ = reader.br.UnreadByte()
			err = reader.parseComment()
			return
		}
		return errBadConfig
	}

	reader._var.format = reader.bufFormat.String()

	return
}

func (reader *reader) parseVariable() (err error) {
	reader.buf.Reset()

	reader.r, _, err = reader.br.ReadRune()
	if err != nil {
		return errVarNameInvalid
	}

	if !unicode.IsLetter(reader.r) {
		return errVarNameInvalid
	}

	reader.bufFormat.Write([]byte{'%', 's'})
	reader.buf.WriteRune(reader.r)

	for {
		reader.r, _, err = reader.br.ReadRune()
		if err != nil {
			break
		}
		if reader.r == tokNewLine {
			reader.bufFormat.WriteRune(reader.r)
			break
		}
		if unicode.IsLetter(reader.r) || unicode.IsDigit(reader.r) ||
			reader.r == tokHyphen || reader.r == tokDot ||
			reader.r == tokUnderscore {
			reader.buf.WriteRune(reader.r)
			continue
		}
		if reader.r == tokHash || reader.r == tokSemiColon {
			_ = reader.br.UnreadRune()

			reader._var.mode = varModeSingle
			reader._var.Key = reader.buf.String()
			reader._var.Value = varValueTrue

			return reader.parseComment()
		}
		if unicode.IsSpace(reader.r) {
			reader.bufFormat.WriteRune(reader.r)

			reader._var.mode = varModeSingle
			reader._var.Key = reader.buf.String()

			return reader.parsePossibleValue()
		}
		if reader.r == tokEqual {
			reader.bufFormat.WriteRune(reader.r)

			reader._var.mode = varModeSingle
			reader._var.Key = reader.buf.String()

			return reader.parseVarValue()
		}
		return errVarNameInvalid
	}

	reader._var.mode = varModeSingle
	reader._var.format = reader.bufFormat.String()
	reader._var.Key = reader.buf.String()
	reader._var.Value = varValueTrue

	return nil
}

//
// parsePossibleValue will check if the next character after space is comment
// or `=`.
//
func (reader *reader) parsePossibleValue() (err error) {
	for {
		reader.b, err = reader.br.ReadByte()
		if err != nil {
			break
		}
		if reader.b == tokNewLine {
			reader.bufFormat.WriteByte(reader.b)
			break
		}
		if reader.b == tokSpace || reader.b == tokTab {
			reader.bufFormat.WriteByte(reader.b)
			continue
		}
		if reader.b == tokHash || reader.b == tokSemiColon {
			_ = reader.br.UnreadByte()
			reader._var.Value = varValueTrue
			return reader.parseComment()
		}
		if reader.b == tokEqual {
			reader.bufFormat.WriteByte(reader.b)
			return reader.parseVarValue()
		}
		return errVarNameInvalid
	}

	reader._var.mode = varModeSingle
	reader._var.format = reader.bufFormat.String()
	reader._var.Value = varValueTrue

	return nil
}

//
// At this point we found `=` on source, and we expect the rest of source will
// be variable value.
//
// (0) Consume leading white-spaces.
//
func (reader *reader) parseVarValue() (err error) {
	reader.buf.Reset()
	reader.bufSpaces.Reset()

	// (0)
	for {
		reader.b, err = reader.br.ReadByte()
		if err != nil {
			reader._var.format = reader.bufFormat.String()
			reader._var.Value = varValueTrue
			return err
		}
		if reader.b == tokSpace || reader.b == tokTab {
			reader.bufFormat.WriteByte(reader.b)
			continue
		}
		if reader.b == tokHash || reader.b == tokSemiColon {
			_ = reader.br.UnreadByte()
			reader._var.Value = varValueTrue
			return reader.parseComment()
		}
		if reader.b == tokNewLine {
			reader.bufFormat.WriteByte(reader.b)
			reader._var.format = reader.bufFormat.String()
			reader._var.Value = varValueTrue
			return nil
		}
		break
	}

	reader._var.mode = varModeValue
	_ = reader.br.UnreadByte()

	var (
		quoted bool
		esc    bool
	)

	for {
		reader.b, err = reader.br.ReadByte()
		if err != nil {
			break
		}

		if esc {
			if reader.b == tokNewLine {
				reader._var.mode = varModeMulti

				reader.valueCommit(true)

				reader.bufFormat.WriteByte(tokNewLine)

				reader.lineNum++
				esc = false
				continue
			}
			if reader.b == tokBackslash || reader.b == tokDoubleQuote {
				reader.valueWriteByte(reader.b)
				esc = false
				continue
			}
			if reader.b == 'b' {
				reader.bufFormat.WriteByte(reader.b)
				reader.buf.WriteByte(tokBackspace)
				esc = false
				continue
			}
			if reader.b == 'n' {
				reader.bufFormat.WriteByte(reader.b)
				reader.buf.WriteByte(tokNewLine)
				esc = false
				continue
			}
			if reader.b == 't' {
				reader.bufFormat.WriteByte(reader.b)
				reader.buf.WriteByte(tokTab)
				esc = false
				continue
			}
			return errValueInvalid
		}
		if reader.b == tokSpace || reader.b == tokTab {
			if quoted {
				reader.valueWriteByte(reader.b)
				continue
			}
			reader.bufFormat.WriteByte(reader.b)
			reader.bufSpaces.WriteByte(reader.b)
			continue
		}
		if reader.b == tokBackslash {
			reader.bufFormat.WriteByte(reader.b)
			esc = true
			continue
		}
		if reader.b == tokDoubleQuote {
			reader.bufFormat.WriteByte(reader.b)
			if quoted {
				quoted = false
			} else {
				quoted = true
			}
			continue
		}
		if reader.b == tokNewLine {
			reader.bufFormat.WriteByte(reader.b)
			break
		}
		if reader.b == tokHash || reader.b == tokSemiColon {
			if quoted {
				reader.valueWriteByte(reader.b)
				continue
			}

			reader.valueCommit(false)

			_ = reader.br.UnreadByte()

			return reader.parseComment()
		}
		reader.valueWriteByte(reader.b)
	}

	if quoted {
		return errValueInvalid
	}

	reader.valueCommit(false)

	reader._var.format = reader.bufFormat.String()

	return nil
}

func (reader *reader) valueCommit(withSpaces bool) {
	val := reader.buf.String()

	if withSpaces {
		val += reader.bufSpaces.String()
	}

	reader._var.Value += val

	reader.buf.Reset()
	reader.bufSpaces.Reset()
}

func (reader *reader) valueWriteByte(b byte) {
	if reader.bufSpaces.Len() > 0 {
		reader.buf.Write(reader.bufSpaces.Bytes())
		reader.bufSpaces.Reset()
	}

	if b == tokPercent {
		reader.bufFormat.Write([]byte{'%', '%'})
	} else {
		reader.bufFormat.WriteByte(b)
	}
	reader.buf.WriteByte(b)
}
