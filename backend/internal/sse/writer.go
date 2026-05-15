package sse

import (
	"fmt"
	"io"
	"strings"
)

const HeartbeatEvent = "HEARTBEAT"

type Frame struct {
	ID    string
	Event string
	Data  string
}

func WriteFrame(w io.Writer, frame Frame) error {
	if frame.ID != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", frame.ID); err != nil {
			return err
		}
	}
	if frame.Event != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", frame.Event); err != nil {
			return err
		}
	}
	data := frame.Data
	if data == "" {
		data = "{}"
	}
	for _, line := range strings.Split(data, "\n") {
		if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func WriteHeartbeat(w io.Writer) error {
	return WriteFrame(w, Frame{
		Event: HeartbeatEvent,
		Data:  "{}",
	})
}
