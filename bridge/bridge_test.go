package bridge

import (
	"fmt"
	"gfep/zptl"
	"testing"
	"time"
)

func sendMsgHandler(buf []byte) {
	fmt.Printf("rx form app: % X\n", buf)
}

func Test698_tm(t *testing.T) {
	c := NewConn("127.0.0.1:8601", []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, zptl.PTL_698_45, time.Minute, sendMsgHandler)
	c.Start()
	<-time.After(time.Second * 10)
	c.Stop()
}
