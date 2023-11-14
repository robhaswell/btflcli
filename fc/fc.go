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
package fc

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/robhaswell/btflcli/msp"
	"go.bug.st/serial"
)

const (
	dfuDevicePrefix     = "Found DFU: "
	internalFlashMarker = "@Internal Flash  /"
)

// FC represents a connection to the flight controller, which can
// handle disconnections and reconnections on its on. Use NewFC()
// to initialize an FC and then call FC.StartUpdating().
type FC struct {
	opts         FCOptions
	msp          *msp.MSP
	Variant      string
	VersionMajor byte
	VersionMinor byte
	VersionPatch byte
	Name         string
	Port         serial.Port
}

type FCOptions struct {
	PortName         string
	BaudRate         int
	Stdout           io.Writer
	EnableDebugTrace bool
}

func (f *FCOptions) stderr() io.Writer {
	return f.Stdout
}

// NewFC returns a new FC using the given port and baud rate. stdout is
// optional and will default to os.Stdout if nil
func NewFC(opts FCOptions) (*FC, error) {
	m, err := msp.New(opts.PortName, opts.BaudRate)
	if err != nil {
		return nil, err
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	fc := &FC{
		opts: opts,
		msp:  m,
		Port: m.Port,
	}
	fc.reset()
	fc.updateInfo()
	return fc, nil
}

func (f *FC) updateInfo() {
	// Send commands to print FC info
	f.msp.WriteCmd(msp.MspAPIVersion)
	f.msp.WriteCmd(msp.MspFCVariant)
	f.msp.WriteCmd(msp.MspFCVersion)
	f.msp.WriteCmd(msp.MspName)

	// Read 4 frames and handle the responses
	m := f.msp
	for ii := 0; ii < 4; ii++ {
		frame, err := m.ReadFrame()
		if err != nil {
			f.Port.Write([]byte("exit\r\n"))
			log.Println(err)
			log.Fatal("MSP communication error, attempting to reset the FC")
		}
		f.handleFrame(frame)
	}
}

func (f *FC) printf(format string, a ...interface{}) (int, error) {
	return fmt.Fprintf(f.opts.Stdout, format, a...)
}

func (f *FC) printInfo() {
	f.printf("Connected to %s %d.%d.%d (%s)\n", f.Variant, f.VersionMajor, f.VersionMinor, f.VersionPatch, f.Name)
}

func (f *FC) handleFrame(fr *msp.MSPFrame) error {
	switch fr.Code {
	case msp.MspAPIVersion:
		f.printf("MSP API version %d.%d (protocol %d)\n", fr.Byte(1), fr.Byte(2), fr.Byte(0))
	case msp.MspFCVariant:
		f.Variant = string(fr.Payload)
	case msp.MspFCVersion:
		f.VersionMajor = fr.Byte(0)
		f.VersionMinor = fr.Byte(1)
		f.VersionPatch = fr.Byte(2)
	case msp.MspName:
		f.Name = string(fr.Payload)
		f.printInfo()
	default:
		f.printf("Unhandled MSP frame %d with payload %v\n", fr.Code, fr.Payload)
	}
	return nil
}

func (f *FC) versionGte(major, minor, patch byte) bool {
	return f.VersionMajor > major || (f.VersionMajor == major && f.VersionMinor > minor) ||
		(f.VersionMajor == major && f.VersionMinor == minor && f.VersionPatch >= patch)
}

func (f *FC) shouldEnableDebugTrace() bool {
	// Only INAV 1.9+ supports DEBUG_TRACE for now
	return f.opts.EnableDebugTrace && f.Variant == "INAV" && f.versionGte(1, 9, 0)
}

func (f *FC) prepareToReboot(fn func(m *msp.MSP) error) error {
	// We want to avoid an EOF from the uart at all costs,
	// so close the current port and open another one to ensure
	// the goroutine reading from the port stops even if the
	// board reboots very fast.
	m := f.msp
	f.msp = nil
	m.Close()
	time.Sleep(time.Second)
	mm, err := msp.New(f.opts.PortName, f.opts.BaudRate)
	if err != nil {
		return err
	}
	err = fn(mm)
	mm.Close()
	return err
}

func (f *FC) reset() {
	f.Variant = ""
	f.VersionMajor = 0
	f.VersionMinor = 0
	f.VersionPatch = 0
}
