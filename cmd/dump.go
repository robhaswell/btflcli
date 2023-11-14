/*
Copyright © 2023 Rob Haswell <rob@haswell.co.uk>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/robhaswell/btflcli/fc"

	"github.com/spf13/cobra"
	"go.bug.st/serial"
)

type MyPIDReceiver struct {
}

const (
	baudRate = 115200
)

// dumpCmd represents the dump command
var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Dump the configuration from a connected flight controller",
	Run:   dumpBoard,
}

func init() {
	rootCmd.AddCommand(dumpCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// dumpCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// dumpCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// Connect to the flight controller over serial, request a dump and save it to a file
func dumpBoard(cmd *cobra.Command, args []string) {
	// Get the most recently connected serial device
	ports, err := serial.GetPortsList()
	if err != nil {
		log.Fatal(err)
	}

	portName := ports[len(ports)-1]

	// Create the fc options to connect to the flight controller
	fcOpts := fc.FCOptions{
		PortName: portName,
		BaudRate: baudRate,
	}

	// Initialise the flight controller connection
	fc, err := fc.NewFC(fcOpts)
	if err != nil {
		log.Fatal(err)
	}
	diffFilename := fmt.Sprintf("%s/%s_%d.%d.%d_DIFF.txt", fc.Name, fc.Variant, fc.VersionMajor, fc.VersionMinor, fc.VersionPatch)
	dumpFilename := fmt.Sprintf("%s/%s_%d.%d.%d_DUMP.txt", fc.Name, fc.Variant, fc.VersionMajor, fc.VersionMinor, fc.VersionPatch)

	// Create a reader utility to read from the flight controller
	reader := bufio.NewReader(fc.Port)
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)

	// Activate the CLI mode
	fc.Port.Write([]byte("#\r\n"))

	for scanner.Scan() {
		// Read a line from the flight controller
		line := scanner.Text()
		if strings.Contains(line, "Entering CLI Mode") {
			break
		}
	}

	// Request a diff
	fc.Port.Write([]byte("diff all\r\n"))
	diffAll := readFcDump(*scanner)

	// Make the output directory if it doesn't exist
	os.MkdirAll(fc.Name, os.ModePerm)

	// Write the diff to a file
	err = os.WriteFile(diffFilename, []byte(diffAll), 0644)
	if err != nil {
		log.Fatal(err)
	}

	// Request a dump
	fc.Port.Write([]byte("dump all\r\n"))
	dumpAll := readFcDump(*scanner)

	// Write the dump to a file
	err = os.WriteFile(dumpFilename, []byte(dumpAll), 0644)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Written files: %s, %s\n", diffFilename, dumpFilename)

	closeFcCli(fc.Port)
}

func readFcDump(scanner bufio.Scanner) string {
	output := ""

	timeStart := time.Now()

	// Read the dump
	for scanner.Scan() {
		// Check for timeout
		if time.Since(timeStart) > 5*time.Second {
			log.Fatal("Timed out waiting for dump")
		}
		// Read a line from the flight controller
		line := scanner.Text()
		output += line + "\r\n"

		// Look to see that we got the whole dump
		if strings.HasSuffix(output, "\r\nsave\r\n") {
			break
		}
	}
	return output
}

func closeFcCli(p serial.Port) {
	p.Write([]byte("exit\r\n"))
}
