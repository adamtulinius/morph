package logging

import (
	"cmp"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"maps"
	"os/exec"
	"slices"
	"strings"
)

type CmdWriter struct {
	Host string
}

func (cmdWriter CmdWriter) Write(inputBytes []byte) (n int, err error) {
	msg := string(inputBytes)
	// A newline will always be included as suffix. Unless removed, this will
	// lead to always printing an empty line at the end.
	msg = strings.TrimSuffix(msg, "\n")
	lines := strings.Split(msg, "\n")

	for _, line := range lines {
		log.Info().Str("Host", cmdWriter.Host).Msg(line)
	}
	return len(inputBytes), nil // This is obviously a lie..
}

func LogCmd(host string, cmd *exec.Cmd) {
	writer := CmdWriter{Host: host}

	cmd.Stdout = writer
	cmd.Stderr = writer

	LogCommand(host, cmd)
}

func LogCommand(host string, cmd *exec.Cmd) {
	// FIXME: This seems to log the executable twice, one with the full path, and one as name only
	zLogCmd := zerolog.Arr().Str(cmd.Path)
	first := true
	for _, arg := range cmd.Args {
		// First arg is the name of the command to run:
		// Skip it, since cmd.Path is the resolved path of the command,
		// and is added already.
		if first {
			first = false
			continue
		}

		zLogCmd.Str(arg)
	}

	log.Info().
		Array("command", zLogCmd).
		Msg(fmt.Sprintf("running command: %s", cmd.String()))
}

func ArrayToZLogArray[T any](elems []T) *zerolog.Array {
	res := zerolog.Arr()

	for _, el := range elems {
		res.Str(fmt.Sprintf("%v", el)) // Hack hack hack
	}

	return res
}

func MapToZLogDict[T cmp.Ordered, U any](elems map[T]U) *zerolog.Event {
	res := zerolog.Dict()

	keys := slices.Sorted(maps.Keys(elems))

	for _, key := range keys {
		value := elems[key]
		res.Str(fmt.Sprintf("%v", key), fmt.Sprintf("%v", value)) // Hack hack hack
	}

	return res
}
