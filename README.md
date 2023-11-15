# btflcli
A command-line interface to dump and restore Betaflight configurations.

## Installation

Copy the binary from the [latest release](https://github.com/robhaswell/btflcli/releases) to a suitable place in your PATH.

## Usage

```
$ btfl --help
This application will connect to a connected Betaflight flight controller and dump the configuration to a file.

It can also restore a configuration from a file to a connected flight controller.

The 'dump' command will create files in a directory matching the craft_name, with filenames matching the version of Betaflight. E.g.:

My Quad/BTFL_4.4.2_DUMP.txt
My Quad/BTFL_4.4.2_DIFF.txt

Use the 'load' command and pass a filename to load the contents of a file to the connected flight controller.

Usage:
  btfl [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  dump        Dump the configuration from a connected flight controller
  help        Help about any command
  load        Load the configuration in the specified file to the connected flight controller

Flags:
  -h, --help   help for btfl

Use "btfl [command] --help" for more information about a command.
```

## Example output

```
$ btfl dump
MSP API version 1.46 (protocol 0)
Connected to BTFL 4.5.0 (M6 HDZero)
Written files: M6 HDZero/BTFL_4.5.0_DIFF.txt, M6 HDZero/BTFL_4.5.0_DUMP.txt
```