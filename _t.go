package main
import ("encoding/hex"; "fmt"; "gfep/zptl")
func main() {
  s := "682100434FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA52DA7D05010140000200007D1B16"
  b, _ := hex.DecodeString(s)
  fmt.Println("len", len(b), "complete", zptl.Ptl698_45CompleteFrameLen(b))
}
