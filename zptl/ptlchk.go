package zptl

type PtlChkfrm struct {
	//超时时间ms
	timeout int64
	//最近接收到数据时标
	rtime int64
	//协议类型
	ptype uint32
	//回调函数
	f func(uint32, []byte, interface{})
	//args: 回调函数的形参
	arg interface{}

	//报文检测长度
	//pos uint32
	//报文检测缓存
	buf []byte
}

//初始化报文检测的方法
func NewPtlChkfrm(ptype uint32, timeout int64, f func(uint32, []byte, interface{}), arg interface{}) *PtlChkfrm {
	//初始化chkfrm属性
	p := &PtlChkfrm{
		timeout: timeout,
		rtime:   0,
		ptype:   ptype,
		f:       f,
		arg:     arg,
		//pos:     0,
		buf: nil,
	}
	return p
}

//报文检测, 返回合法报文数量
func (p *PtlChkfrm) Chkfrm(data []byte) int32 {
	var cnt int32 = 0

	if p.ptype == 0 {
		return 0
	}

	if p.ptype == PTL_RAW {
		if p.f != nil {
			p.f(PTL_RAW, data, p.arg)
		}
		return 1
	}

	if GetTick()-p.rtime > p.timeout {
		if len(p.buf) > 0 {
			p.buf = p.buf[1:]
		} else {
			p.Reset()
		}
	}

	if len(data) <= 0 {
		return cnt
	}

	p.rtime = GetTick()
	if p.buf == nil {
		p.buf = make([]byte, 0, PmaxPtlFrameLen)
		if p.buf == nil {
			return cnt
		}
	}

	//切片叠加
	p.buf = append(p.buf, data...)

	for len(p.buf) > 0 {
		pos, rlen, ptype := PtlCheck(p.ptype, p.buf)
		if pos < 0 {
			p.buf = p.buf[0:0]
			return cnt //68,88,98都找不到, 不需要申请空间
		}
		if pos > 0 {
			p.buf = p.buf[pos:]
		}
		if rlen > 0 {
			if p.f != nil {
				p.f(ptype, p.buf[:rlen], p.arg)
			}
			p.buf = p.buf[rlen:]
		}
	}

	return cnt
}

//复位
func (p *PtlChkfrm) Reset() {
	// p.pos = 0
	p.rtime = 0
	//可以考虑释放buf
	p.buf = nil
}
