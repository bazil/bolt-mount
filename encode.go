package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
)

const FragSeparator = ':'

func DecodeKey(quoted string) ([]byte, error) {
	var key []byte
	for _, frag := range strings.Split(quoted, string(FragSeparator)) {
		if frag == "" {
			return nil, fmt.Errorf("quoted key cannot have empty fragment: %s", quoted)
		}
		switch {
		case strings.HasPrefix(frag, "@"):
			f, err := hex.DecodeString(frag[1:])
			if err != nil {
				return nil, err
			}
			key = append(key, f...)
		default:
			key = append(key, frag...)
		}
	}
	return key, nil
}

func isSafe(r rune) bool {
	if r > unicode.MaxASCII {
		return false
	}
	if unicode.IsLetter(r) || unicode.IsNumber(r) {
		return true
	}
	switch r {
	case FragSeparator:
		return false
	case '.', ',', '-', '_':
		return true
	}
	return false
}

const prettyTheshold = 2

func EncodeKey(key []byte) string {
	// we do sloppy work and process safe bytes only at the beginning
	// and end; this avoids many false positives in large binary data

	var left, right []byte
	var middle string

	if key[0] != '.' {
		mid := bytes.TrimLeftFunc(key, isSafe)
		if len(key)-len(mid) > prettyTheshold {
			left = key[:len(key)-len(mid)]
			key = mid
		}
	}

	{
		mid := bytes.TrimRightFunc(key, isSafe)
		if len(mid) == 0 && len(key) > 0 && key[0] == '.' {
			// don't let right safe zone reach all the way to leading dot
			mid = key[:1]
		}
		if len(key)-len(mid) > prettyTheshold {
			right = key[len(mid):]
			key = mid
		}
	}

	if len(key) > 0 {
		middle = "@" + hex.EncodeToString(key)
	}

	return strings.Trim(
		string(left)+string(FragSeparator)+middle+string(FragSeparator)+string(right),
		string(FragSeparator),
	)
}
