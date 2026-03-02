package proton

import (
	"encoding/binary"
	"fmt"
	"unicode/utf16"
)

// MS-SHLLINK binary format constants.
const (
	lnkHeaderSize           = 76
	lnkFlagHasIDList        = 0x01
	lnkFlagHasLinkInfo      = 0x02
	lnkInfoFlagLocalPath    = 0x01
	lnkMinLinkInfoSize      = 28
	lnkUnicodeInfoHeaderMin = 36
)

// parseLNKTarget extracts the local Windows target path from a .lnk file's
// binary content. Returns an error if the file is not a valid LNK or has no
// local target path.
func parseLNKTarget(data []byte) (string, error) {
	if len(data) < lnkHeaderSize {
		return "", fmt.Errorf("file too small to be an LNK")
	}

	// Byte 0: HeaderSize must be 0x4C.
	if binary.LittleEndian.Uint32(data[0:4]) != lnkHeaderSize {
		return "", fmt.Errorf("not an LNK file (bad header size)")
	}

	linkFlags := binary.LittleEndian.Uint32(data[20:24])
	offset := lnkHeaderSize

	// Skip the optional ID list.
	if linkFlags&lnkFlagHasIDList != 0 {
		if offset+2 > len(data) {
			return "", fmt.Errorf("truncated IDList size")
		}
		idListSize := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
		offset += 2 + idListSize
	}

	if linkFlags&lnkFlagHasLinkInfo == 0 {
		return "", fmt.Errorf("LNK has no LinkInfo block")
	}

	if offset+lnkMinLinkInfoSize > len(data) {
		return "", fmt.Errorf("truncated LinkInfo block")
	}

	liStart := offset
	liSize := int(binary.LittleEndian.Uint32(data[liStart : liStart+4]))
	if liStart+liSize > len(data) {
		return "", fmt.Errorf("LinkInfo size exceeds file")
	}

	liHeaderSize := int(binary.LittleEndian.Uint32(data[liStart+4 : liStart+8]))
	liFlags := binary.LittleEndian.Uint32(data[liStart+8 : liStart+12])

	if liFlags&lnkInfoFlagLocalPath == 0 {
		return "", fmt.Errorf("LNK target is not a local path")
	}

	// Prefer the Unicode path when available (LinkInfoHeaderSize >= 36).
	if liHeaderSize >= lnkUnicodeInfoHeaderMin && liStart+32 <= len(data) {
		uniOff := int(binary.LittleEndian.Uint32(data[liStart+28 : liStart+32]))
		if uniOff > 0 {
			if path, err := readUTF16LEString(data, liStart+uniOff, liStart+liSize); err == nil && path != "" {
				return path, nil
			}
		}
	}

	// Fall back to ASCII (ANSI code-page) path.
	localBasePathOff := int(binary.LittleEndian.Uint32(data[liStart+16 : liStart+20]))
	path := readCString(data, liStart+localBasePathOff, liStart+liSize)
	if path == "" {
		return "", fmt.Errorf("empty local base path in LNK")
	}
	return path, nil
}

// readCString reads a null-terminated ASCII string starting at offset,
// not exceeding limit.
func readCString(data []byte, offset, limit int) string {
	if offset < 0 || offset >= limit || limit > len(data) {
		return ""
	}
	end := offset
	for end < limit && data[end] != 0 {
		end++
	}
	return string(data[offset:end])
}

// readUTF16LEString reads a null-terminated UTF-16 LE string starting at
// offset, not exceeding limit.
func readUTF16LEString(data []byte, offset, limit int) (string, error) {
	if offset < 0 || offset >= limit || limit > len(data) {
		return "", fmt.Errorf("UTF-16 offset out of range")
	}
	var units []uint16
	for i := offset; i+1 < limit; i += 2 {
		u := binary.LittleEndian.Uint16(data[i : i+2])
		if u == 0 {
			break
		}
		units = append(units, u)
	}
	return string(utf16.Decode(units)), nil
}
