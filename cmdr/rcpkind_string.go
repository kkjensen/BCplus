// Code generated by "stringer -type RcpKind"; DO NOT EDIT.

package cmdr

import "strconv"

const _RcpKind_name = "RcpSynthRcpEngie"

var _RcpKind_index = [...]uint8{0, 8, 16}

func (i RcpKind) String() string {
	if i < 0 || i >= RcpKind(len(_RcpKind_index)-1) {
		return "RcpKind(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _RcpKind_name[_RcpKind_index[i]:_RcpKind_index[i+1]]
}