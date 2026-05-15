package sse

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrInvalidEventID = errors.New("invalid SSE event id")
	eventIDPattern    = regexp.MustCompile(`^evt_([0-9]+)$`)
)

func CursorFromEventIDs(headerValue string, queryValue string) (uint64, error) {
	if value := strings.TrimSpace(headerValue); value != "" {
		return ParseEventID(value)
	}
	if value := strings.TrimSpace(queryValue); value != "" {
		return ParseEventID(value)
	}
	return 0, nil
}

func ParseEventID(value string) (uint64, error) {
	value = strings.TrimSpace(value)
	matches := eventIDPattern.FindStringSubmatch(value)
	if len(matches) != 2 {
		return 0, ErrInvalidEventID
	}
	sequence, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return 0, ErrInvalidEventID
	}
	return sequence, nil
}
