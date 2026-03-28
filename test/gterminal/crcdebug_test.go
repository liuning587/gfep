package main

import (
	"encoding/hex"
	"testing"

	"gfep/zptl"
)

// 日志里主站下发的广播抄读时间原始 hex（用户抓包）
const logBroadcastHex = "682100434FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA52DA7D05010140000200007D1B16"

func TestLogBroadcastFrameValid698(t *testing.T) {
	b, err := hex.DecodeString(logBroadcastHex)
	if err != nil {
		t.Fatal(err)
	}
	n := zptl.Ptl698_45CompleteFrameLen(b)
	if n > 0 {
		t.Log("log frame passes strict zptl check")
	} else {
		t.Logf("log frame strict check=%d (expected: common tools use wrong HCS/FCS layout)", n)
	}
	if !isBroadcastTimeReadLayout(b) {
		t.Fatal("log frame should match broadcast time layout for gterminal loose parse")
	}
}
