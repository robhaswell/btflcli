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
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/robhaswell/btflcli/fc"
	"github.com/spf13/cobra"
	"go.bug.st/serial"
)

// loadCmd represents the load command
var loadCmd = &cobra.Command{
	Use:   "load",
	Short: "Load the configuration in the specified file to the connected flight controller",
	Args:  cobra.ExactArgs(1),
	Run:   loadFile,
}

func init() {
	rootCmd.AddCommand(loadCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// loadCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// loadCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func loadFile(cmd *cobra.Command, args []string) {
	inputFile := args[0]
	fileContents, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatal(err)
	}

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
	p := fc.Port

	// Create a reader utility to read from the flight controller
	scanner := bufio.NewScanner(bufio.NewReader(p))
	scanner.Split(bufio.ScanLines)

	fileScanner := bufio.NewScanner(bytes.NewReader(fileContents))
	fileScanner.Split(bufio.ScanLines)

	// Activate the CLI mode
	p.Write([]byte("#\r\n"))

	for scanner.Scan() {
		// Read a line from the flight controller
		line := scanner.Text()
		if strings.Contains(line, "Entering CLI Mode") {
			// Read one more blank link from the FC.
			scanner.Scan()
			break
		}
	}

	p.SetReadTimeout(100 * time.Millisecond)

	// Send the file contents to the flight controller
	for fileScanner.Scan() {
		line := fileScanner.Text()
		if line == "" {
			continue
		}
		p.Write([]byte(line + "\r\n"))
		fmt.Println(line)

		// Read the response from the flight controller
		for {
			buf := make([]byte, 1024)
			n, err := p.Read(buf)
			if err != nil {
				break
			}
			if n == 0 {
				break
			}
		}
	}

	// Send save command just in case
	p.Write([]byte("save\r\n"))

	// Read the response from the flight controller
	for {
		buf := make([]byte, 1024)
		n, err := p.Read(buf)
		if err != nil {
			break
		}
		if n == 0 {
			break
		}
	}

	// The flight controller should reboot so no need to close the connection
	fmt.Println("\n\nConfiguration loaded")

}
