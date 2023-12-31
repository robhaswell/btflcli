/*
MIT License

Copyright (c) 2018 Alberto Garcia Hierro <alberto@garciahierro.com>

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
package msp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"

	"go.bug.st/serial"
)

const (
	MspAPIVersion = 1
	MspFCVariant  = 2
	MspFCVersion  = 3
	MspBoardInfo  = 4
	MspBuildInfo  = 5

	MspName = 10

	MspFeature    = 36
	MspSetFeature = 37

	MspCFSerialConfig    = 54
	MspSetCFSerialConfig = 55

	MspRXMap = 64

	MspReboot = 68

	MspPID = 112

	MspSetRawRC = 200

	MspSetPID = 202

	MspEepromWrite = 250

	MspDebugMsg = 253
)

const (
	MspFCFeatureDebugTrace = 1 << 31
)

const (
	SerialFunctionMSP        = 1 << 0
	SerialFunctionDebugTrace = 1 << 15
)

func mspV1Encode(cmd byte, data []byte) []byte {
	var payloadLength byte
	if len(data) > 0 {
		payloadLength = byte(len(data))
	}
	var buf bytes.Buffer
	buf.WriteByte('$')
	buf.WriteByte('M')
	buf.WriteByte('<')
	buf.WriteByte(payloadLength)
	buf.WriteByte(cmd)
	if payloadLength > 0 {
		buf.Write(data)
	}
	crc := byte(0)
	for _, v := range buf.Bytes()[3:] {
		crc ^= v
	}
	buf.WriteByte(crc)
	return buf.Bytes()
}

func mspV2Encode(cmd byte, totalLength int) []byte {
	var payloadLength byte
	if totalLength > 6 {
		payloadLength = byte(totalLength) - 9
	}
	var buf bytes.Buffer
	buf.WriteByte('$')
	buf.WriteByte('X')
	buf.WriteByte('<')
	buf.WriteByte(0)
	buf.WriteByte(cmd)
	buf.WriteByte(0)
	buf.WriteByte(byte(payloadLength))
	buf.WriteByte(0)
	for ii := byte(0); ii < payloadLength; ii++ {
		buf.WriteByte(0)
	}
	crc := byte(0)
	for _, v := range buf.Bytes()[3:] {
		crc = crc8DvbS2(crc, v)
	}
	buf.WriteByte(crc)
	return buf.Bytes()
}

func crc8DvbS2(crc, a byte) byte {
	crc ^= a
	for ii := 0; ii < 8; ii++ {
		if (crc & 0x80) != 0 {
			crc = (crc << 1) ^ 0xD5
		} else {
			crc = crc << 1
		}
	}
	return crc
}

type MSP struct {
	portName string
	baudRate int
	Port     serial.Port
}

type MSPFrame struct {
	Code       uint16
	Payload    []byte
	payloadPos int
}

func (f *MSPFrame) Byte(idx int) byte {
	return f.Payload[idx]
}

// Reads out from the frame Payload and advances the payload
// position pointer by the size of the variable pointed by out.
func (f *MSPFrame) Read(out interface{}) error {
	switch x := out.(type) {
	case *uint8:
		if f.BytesRemaining() < 1 {
			return io.EOF
		}
		*x = f.Payload[f.payloadPos]
		f.payloadPos++
	case *uint16:
		if f.BytesRemaining() < 2 {
			return io.EOF
		}
		*x = binary.LittleEndian.Uint16(f.Payload[f.payloadPos:])
		f.payloadPos += 2
	case *uint32:
		if f.BytesRemaining() < 4 {
			return io.EOF
		}
		*x = binary.LittleEndian.Uint32(f.Payload[f.payloadPos:])
		f.payloadPos += 4
	default:
		v := reflect.ValueOf(out)
		if v.Kind() == reflect.Ptr && v.Elem().Kind() == reflect.Struct {
			elem := v.Elem()
			for ii := 0; ii < elem.NumField(); ii++ {
				field := elem.Field(ii).Addr()
				if err := f.Read(field.Interface()); err != nil {
					return err
				}
			}
			return nil
		}
		if v.Kind() == reflect.Slice {
			for ii := 0; ii < v.Len(); ii++ {
				elem := v.Index(ii)
				if err := f.Read(elem.Addr().Interface()); err != nil {
					return err
				}
			}
			return nil
		}
		panic(fmt.Errorf("can't decode MSP payload into type %T", out))
	}
	return nil
}

func (f *MSPFrame) BytesRemaining() int {
	return len(f.Payload) - f.payloadPos
}

type MSPError interface {
	error
	IsMSPError() bool
}

type mspChecksumErr struct {
	code             uint16
	payload          []byte
	checksum         uint8
	expectedChecksum uint8
}

func (e *mspChecksumErr) Checksum() uint8         { return e.checksum }
func (e *mspChecksumErr) ExpectedChecksum() uint8 { return e.expectedChecksum }
func (e *mspChecksumErr) IsMSPError() bool        { return true }
func (e *mspChecksumErr) Error() string {
	return fmt.Sprintf("invalid CRC 0x%02x, expecting 0x%02x in cmd %v with payload %v",
		e.checksum, e.expectedChecksum, e.code, e.payload)
}

type mspOOBErr struct {
	b byte
}

func (e *mspOOBErr) IsMSPError() bool { return true }
func (e *mspOOBErr) Error() string {
	return fmt.Sprintf("out of band MSP byte 0x%02x", e.b)
}

func New(portName string, baudRate int) (*MSP, error) {
	mode := &serial.Mode{
		BaudRate: baudRate,
	}
	port, err := serial.Open(portName, mode)
	if err != nil {
		return nil, err
	}
	return &MSP{
		portName: portName,
		baudRate: baudRate,
		Port:     port,
	}, nil
}

func (m *MSP) encodeArgs(w *bytes.Buffer, args ...interface{}) error {
	for _, arg := range args {
		switch x := arg.(type) {
		case uint8:
			w.WriteByte(x)
		case uint16:
			binary.Write(w, binary.LittleEndian, x)
		case uint32:
			binary.Write(w, binary.LittleEndian, x)
		default:
			v := reflect.ValueOf(arg)
			if v.Kind() == reflect.Slice {
				for ii := 0; ii < v.Len(); ii++ {
					if err := m.encodeArgs(w, v.Index(ii).Interface()); err != nil {
						return err
					}
				}
				return nil
			}
			if v.Kind() == reflect.Struct {
				for ii := 0; ii < v.NumField(); ii++ {
					if err := m.encodeArgs(w, v.Field(ii).Interface()); err != nil {
						return err
					}
				}
				return nil
			}
			panic(fmt.Errorf("can't encode MSP value of type %T", arg))
		}
	}
	return nil
}

func (m *MSP) WriteCmd(cmd uint16, args ...interface{}) (int, error) {
	var buf bytes.Buffer
	if err := m.encodeArgs(&buf, args...); err != nil {
		return -1, err
	}
	data := buf.Bytes()
	frame := mspV1Encode(byte(cmd), data)
	return m.Port.Write(frame)
}

func (m *MSP) readMSPV1Frame() (*MSPFrame, error) {
	buf := make([]byte, 3)
	if _, err := m.Port.Read(buf); err != nil {
		return nil, err
	}
	if buf[0] != '<' && buf[0] != '>' {
		return nil, fmt.Errorf("invalid MSP direction char 0x%02x", buf[0])
	}
	ccrc := byte(0)
	ccrc ^= buf[1]
	ccrc ^= buf[2]
	var payload []byte
	payloadLength := int(buf[1])
	cmd := buf[2]
	if payloadLength > 0 {
		payload = make([]byte, payloadLength)
		if _, err := io.ReadFull(m.Port, payload); err != nil {
			return nil, err
		}
		for _, b := range payload {
			ccrc ^= b
		}
	}
	buf = buf[:1]
	if _, err := m.Port.Read(buf); err != nil {
		return nil, err
	}
	crc := buf[0]
	if crc != ccrc {
		return nil, &mspChecksumErr{
			code:             uint16(cmd),
			payload:          payload,
			checksum:         crc,
			expectedChecksum: ccrc,
		}
	}
	return &MSPFrame{
		Code:       uint16(cmd),
		Payload:    payload,
		payloadPos: 0,
	}, nil
}

func (m *MSP) readMSPV2Frame() (*MSPFrame, error) {
	buf := make([]byte, 6)
	if _, err := m.Port.Read(buf); err != nil {
		return nil, err
	}
	if buf[0] != '<' && buf[0] != '>' {
		return nil, fmt.Errorf("invalid MSP direction char 0x%02x", buf[0])
	}
	// flags := buf[1]
	code := uint16(buf[2]) | uint16(buf[3])<<8
	payloadLength := int(uint16(buf[4]) | uint16(buf[5])<<8)
	var payload []byte
	if payloadLength > 0 {
		payload = make([]byte, payloadLength)
		if _, err := io.ReadFull(m.Port, payload); err != nil {
			return nil, err
		}
	}

	buf = make([]byte, 1)
	if _, err := m.Port.Read(buf); err != nil {
		return nil, err
	}
	// crc := buf[0]
	return &MSPFrame{
		Code:       code,
		Payload:    payload,
		payloadPos: 0,
	}, nil
}

func (m *MSP) ReadFrame() (*MSPFrame, error) {
	port := m.Port
	if port == nil {
		return nil, io.EOF
	}
	buf := make([]byte, 1)
	for {
		_, err := port.Read(buf)
		if err != nil {
			return nil, err
		}
		if buf[0] == '$' {
			// Frame start
			break
		}
		return nil, &mspOOBErr{b: buf[0]}
	}
	_, err := port.Read(buf)
	if err != nil {
		return nil, err
	}
	switch buf[0] {
	case 'M':
		return m.readMSPV1Frame()
	case 'X':
		return m.readMSPV2Frame()
	default:
		return nil, fmt.Errorf("unknown MSP char %c", buf[0])
	}
}

// RebootIntoBootloader reboots the board into bootloader mode
func (m *MSP) RebootIntoBootloader() (int, error) {
	// reboot_character is 'R' by default, but it can be changed
	// TODO: Retrieve it if possible (in inav it can be done via MSPv2)
	return m.Port.Write([]byte{'R'})
}

// Close closes the underlying serial port. Note that reading from or
// writing to a closed MSP will cause a panic.
func (m *MSP) Close() error {
	var err error
	if m.Port != nil {
		if err = m.Port.Close(); err == nil {
			m.Port = nil
		}
	}
	return err
}
