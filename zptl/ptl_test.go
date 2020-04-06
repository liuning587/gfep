package zptl

import (
	"fmt"
	"testing"
)

func cb(ptype uint32, data []byte, arg interface{}) {
	fmt.Println("ptype: ", ptype)
	PrintBuf(0, data)
	fmt.Println("TSA: ", Ptl698_45AddrGet(data), Ptl698_45AddrStr(Ptl698_45AddrGet(data)))
}

func TestPtlChkfrm_00(t *testing.T) {
	//创建一个报文检测对象
	chkfrm := NewPtlChkfrm(PTL_698_45, 1000, cb, nil)

	var cnt int32
	// cnt = chkfrm.Chkfrm(Str2hex("68"))
	// fmt.Println("cnt: ", cnt)
	packet := Str2hex("68 21 00 C3 05 11 11 11 11 01 00 CC 38 81 85 01 01 40 00 02 00 01 1C 07 E4 04 03 0F 02 0A 00 00 72 99 16")
	rlen, ptype := PtlIsVaild(PTL_698_45, packet)
	fmt.Println("rlen: ", rlen, " ptype: ", ptype)

	cnt = chkfrm.Chkfrm(Str2hex("68 21 00 C3 05 11 11 11 11 01 00 CC 38 81 85 01 01 40 00 02 00 01 1C 07 E4 04 03 0F 02 0A 00 00 72 99 16"))
	fmt.Println("cnt: ", cnt)
}

// package main

// import "fmt"

// func main() {
//     x := make(map[string][]string)

//     x["key"] = append(x["key"], "value")
//     x["key"] = append(x["key"], "value1")

//     fmt.Println(x["key"][0])
//     fmt.Println(x["key"][1])
// }
