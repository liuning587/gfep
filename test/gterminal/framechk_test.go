package main

import (
	"encoding/hex"
	"testing"

	"gfep/zptl"
)

func TestUserBroadcastTimeFrames(t *testing.T) {
	// L=0x21：HCS 在 22–23，用户数据仅 24–31 共 8 字节；32–33 为 FCS（用户串里 02 后的 00 00 实为 FCS 低高字节）
	req := make([]byte, 35)
	req[0] = 0x68
	req[1] = 0x21
	req[2] = 0x00
	req[3] = 0x43
	req[4] = 0x4F
	for i := 5; i < 21; i++ {
		req[i] = 0xAA
	}
	req[21] = 0x52
	copy(req[24:32], []byte{0xDA, 0x7D, 0x05, 0x01, 0x01, 0x40, 0x00, 0x02})
	req[34] = 0x16
	if err := recalc698CRCs(req); err != nil {
		t.Fatal(err)
	}
	n := zptl.Ptl698_45CompleteFrameLen(req)
	if n != 35 {
		t.Fatalf("req complete len want 35 got %d", n)
	}
	if !isBroadcastTimeReadLayout(req) {
		t.Fatal("isBroadcastTimeReadLayout should match built broadcast read-time frame")
	}
	respHex := "682100C30501000000009452624B85010140000200011C07EB040B07111000003B4316"
	resp, err := hex.DecodeString(respHex)
	if err != nil {
		t.Fatal(err)
	}
	if err := recalc698CRCs(resp); err != nil {
		t.Fatal(err)
	}
	n2 := zptl.Ptl698_45CompleteFrameLen(resp)
	if n2 <= 0 {
		t.Fatalf("resp not valid: %d", n2)
	}
}
