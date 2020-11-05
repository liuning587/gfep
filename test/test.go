package main

import (
	"fmt"
	"net"
	"sync"
	"time"
)

var wg sync.WaitGroup

func send(c net.Conn, cNo int) {
	sbuf := make([]byte, 32)
	sbuf[0] = 0x68
	sbuf[1] = 0x1E
	sbuf[2] = 0x00
	sbuf[3] = 0x81
	sbuf[4] = 0x05
	sbuf[5] = 0x15
	sbuf[6] = 0x61
	sbuf[7] = 0x50
	sbuf[8] = 0x88
	sbuf[9] = 0x58
	sbuf[10] = 0x01
	sbuf[11] = 0x00
	sbuf[12] = 0xDA
	sbuf[13] = 0x0E
	sbuf[14] = 0x01
	sbuf[15] = 0x00
	sbuf[16] = 0x00
	sbuf[17] = 0x00
	sbuf[18] = 0x3C
	sbuf[19] = 0x07
	sbuf[20] = 0xE4
	sbuf[21] = 0x03
	sbuf[22] = 0x19
	sbuf[23] = 0x03
	sbuf[24] = 0x0B
	sbuf[25] = 0x1D
	sbuf[26] = 0x01
	sbuf[27] = 0x03
	sbuf[28] = 0x84
	sbuf[29] = 0x9C
	sbuf[30] = 0x10
	sbuf[31] = 0x16

	for {
		//客户端请求数据写入 conn，并传输
		_, err := c.Write([]byte(sbuf))
		if err != nil {
			fmt.Printf("客户端发送数据失败 %d, %s\n", cNo, err)
			c.Close()
			wg.Done()
			break
		}
		//fmt.Printf("客户端发送: %d, %d, %s\n", cNo, cnt, hextostr(sbuf[:cnt], " "))
		time.Sleep(1 * time.Second)
	}
}

func rece(c net.Conn, cNo int) {

	//接收缓存
	rbuf := make([]byte, 1024)

	for {
		//服务器端返回的数据写入空buf
		_, err := c.Read(rbuf)
		if err != nil {
			fmt.Printf("客户端接收数据失败: %d, %s\n", cNo, err)
			c.Close()
			wg.Done()
			break
		}
		//fmt.Printf("服务器端回复: %d, %d, %s\n", cNo, cnt, hextostr(rbuf[:cnt], " "))
	}
}

// ClientSocket 客户端连接
func ClientSocket() {
	for i := 0; i < 10000; i++ {
		wg.Add(1)
		go func(i int) {
			time.Sleep(time.Duration(i*10000) * time.Microsecond)
			conn, err := net.Dial("tcp", "127.0.0.1:20083")
			if err != nil {
				fmt.Println("客户端建立连接失败", i, err)
				return
			}
			fmt.Println("客户端建立连接OK", i)

			wg.Add(1)
			go send(conn, i)

			wg.Add(1)
			go rece(conn, i)
		}(i)
	}
	wg.Wait()
}

func main() {

	fmt.Printf("客户端测试\n")

	ClientSocket()

	wg.Wait()
}
