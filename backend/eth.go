package main

import (
    "bytes"
    "crypto/rand"
    "encoding/binary"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "math/big"
    "net/http"
    "os"
    "strings"
)

const bscRPCURL = "https://bsc-testnet-dataseed.bnbchain.org"

var (
    secpP  = mustBig("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F")
    secpN  = mustBig("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141")
    secpGx = mustBig("79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798")
    secpGy = mustBig("483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8")
)

func mustBig(s string) *big.Int {
    n, ok := new(big.Int).SetString(s, 16)
    if !ok { panic("invalid secp256k1 constant") }
    return n
}

type secpPoint struct { x, y *big.Int; inf bool }

func mod(v *big.Int) *big.Int { return new(big.Int).Mod(v, secpP) }

func pointAdd(a, b secpPoint) secpPoint {
    if a.inf { return b }
    if b.inf { return a }
    if a.x.Cmp(b.x) == 0 {
        sumY := mod(new(big.Int).Add(a.y, b.y))
        if sumY.Sign() == 0 { return secpPoint{inf:true} }
        num := new(big.Int).Mul(a.x, a.x)
        num.Mul(num, big.NewInt(3))
        den := new(big.Int).Lsh(new(big.Int).Set(a.y), 1)
        den.Mod(den, secpP)
        den.ModInverse(den, secpP)
        lambda := mod(new(big.Int).Mul(num, den))
        x3 := mod(new(big.Int).Sub(new(big.Int).Mul(lambda, lambda), new(big.Int).Add(a.x, b.x)))
        y3 := mod(new(big.Int).Sub(new(big.Int).Mul(lambda, new(big.Int).Sub(a.x, x3)), a.y))
        return secpPoint{x:x3,y:y3}
    }
    num := new(big.Int).Sub(b.y, a.y)
    den := new(big.Int).Sub(b.x, a.x)
    den.Mod(den, secpP)
    den.ModInverse(den, secpP)
    lambda := mod(new(big.Int).Mul(num, den))
    x3 := mod(new(big.Int).Sub(new(big.Int).Sub(new(big.Int).Mul(lambda, lambda), a.x), b.x))
    y3 := mod(new(big.Int).Sub(new(big.Int).Mul(lambda, new(big.Int).Sub(a.x, x3)), a.y))
    return secpPoint{x:x3,y:y3}
}

func scalarMult(p secpPoint, k *big.Int) secpPoint {
    result := secpPoint{inf:true}
    for i := k.BitLen()-1; i >= 0; i-- {
        result = pointAdd(result, result)
        if k.Bit(i) == 1 { result = pointAdd(result, p) }
    }
    return result
}

func scalarBaseMult(k *big.Int) secpPoint {
    return scalarMult(secpPoint{x:secpGx,y:secpGy}, k)
}

func parsePrivateKey(s string) (*big.Int,error) {
    s = strings.TrimPrefix(strings.TrimSpace(s),"0x")
    if len(s) != 64 { return nil,fmt.Errorf("private key must be 32 bytes") }
    b,err := hex.DecodeString(s); if err != nil { return nil,fmt.Errorf("invalid private key") }
    d:=new(big.Int).SetBytes(b)
    if d.Sign()<=0 || d.Cmp(secpN)>=0 { return nil,fmt.Errorf("private key out of range") }
    return d,nil
}

func ethAddressFromPrivate(d *big.Int) string {
    pub:=scalarBaseMult(d)
    raw:=make([]byte,64); pub.x.FillBytes(raw[:32]); pub.y.FillBytes(raw[32:])
    hash:=keccak256(raw)
    return "0x"+hex.EncodeToString(hash[12:])
}

func randomScalar() (*big.Int,error) {
    buf:=make([]byte,32)
    for {
        if _,err:=rand.Read(buf); err!=nil { return nil,err }
        k:=new(big.Int).SetBytes(buf)
        if k.Sign()>0 && k.Cmp(secpN)<0 { return k,nil }
    }
}

func signDigest(d *big.Int,digest []byte)(r,s *big.Int,recovery byte,err error){
    z:=new(big.Int).SetBytes(digest)
    for {
        k,e:=randomScalar(); if e!=nil{return nil,nil,0,e}
        R:=scalarBaseMult(k)
        r=new(big.Int).Mod(new(big.Int).Set(R.x),secpN); if r.Sign()==0{continue}
        kinv:=new(big.Int).ModInverse(k,secpN)
        s=new(big.Int).Mul(r,d); s.Add(s,z); s.Mul(s,kinv); s.Mod(s,secpN); if s.Sign()==0{continue}
        recovery=byte(R.y.Bit(0)); if R.x.Cmp(secpN)>=0{recovery|=2}
        halfN:=new(big.Int).Rsh(new(big.Int).Set(secpN),1)
        if s.Cmp(halfN)>0{s.Sub(secpN,s); recovery^=1}
        return r,s,recovery,nil
    }
}

var keccakRC=[24]uint64{1,0x8082,0x800000000000808A,0x8000000080008000,0x808B,0x80000001,0x8000000080008081,0x8000000000008009,0x8A,0x88,0x80008009,0x8000000A,0x8000808B,0x800000000000008B,0x8000000000008089,0x8000000000008003,0x8000000000008002,0x8000000000000080,0x800A,0x800000008000000A,0x80008081,0x8000000000008080,0x80000001,0x8000000080008008}
var keccakRot=[25]uint{0,1,62,28,27,36,44,6,55,20,3,10,43,25,39,41,45,15,21,8,18,2,61,56,14}

func keccakROL(x uint64,n uint)uint64{if n==0{return x};return x<<n|x>>(64-n)}

func keccakF(a *[25]uint64){
    var b [25]uint64
    for round:=0;round<24;round++{
        var c,d [5]uint64
        for x:=0;x<5;x++{c[x]=a[x]^a[x+5]^a[x+10]^a[x+15]^a[x+20]}
        for x:=0;x<5;x++{d[x]=c[(x+4)%5]^keccakROL(c[(x+1)%5],1);for y:=0;y<5;y++{a[x+5*y]^=d[x]}}
        for x:=0;x<5;x++{for y:=0;y<5;y++{b[y+5*((2*x+3*y)%5)]=keccakROL(a[x+5*y],keccakRot[x+5*y])}}
        for x:=0;x<5;x++{for y:=0;y<5;y++{a[x+5*y]=b[x+5*y]^((^b[(x+1)%5+5*y])&b[(x+2)%5+5*y])}}
        a[0]^=keccakRC[round]
    }
}

func keccak256(data []byte)[]byte{
    const rate=136
    var state [25]uint64
    for len(data)>=rate{for i:=0;i<rate/8;i++{state[i]^=binary.LittleEndian.Uint64(data[i*8:])};keccakF(&state);data=data[rate:]}
    var block [rate]byte;copy(block[:],data);block[len(data)]^=1;block[rate-1]^=0x80
    for i:=0;i<rate/8;i++{state[i]^=binary.LittleEndian.Uint64(block[i*8:])};keccakF(&state)
    out:=make([]byte,32);for i:=0;i<4;i++{binary.LittleEndian.PutUint64(out[i*8:],state[i])};return out
}

func rlpBytes(b []byte)[]byte{
    if len(b)==1 && b[0]<0x80{return append([]byte(nil),b...)}
    if len(b)<=55{return append([]byte{byte(0x80+len(b))},b...)}
    length:=intToMinimalBytes(len(b));return append(append([]byte{byte(0xb7+len(length))},length...),b...)
}
func rlpUint(n *big.Int)[]byte{if n==nil||n.Sign()==0{return []byte{0x80}};return rlpBytes(n.Bytes())}
func rlpList(items ...[]byte)[]byte{payload:=bytes.Join(items,nil);if len(payload)<=55{return append([]byte{byte(0xc0+len(payload))},payload...)};length:=intToMinimalBytes(len(payload));return append(append([]byte{byte(0xf7+len(length))},length...),payload...)}
func intToMinimalBytes(n int)[]byte{if n==0{return []byte{0}};var out []byte;for n>0{out=append([]byte{byte(n)},out...);n>>=8};return out}
func pad32(n *big.Int)[]byte{out:=make([]byte,32);n.FillBytes(out);return out}

func buildTransferData(toWallet string,amount *big.Int)([]byte,error){
    addr:=strings.TrimPrefix(strings.TrimSpace(toWallet),"0x");if len(addr)!=40{return nil,fmt.Errorf("wallet address is invalid")}
    addressBytes,err:=hex.DecodeString(addr);if err!=nil{return nil,fmt.Errorf("wallet address is invalid")}
    data:=make([]byte,0,68);data=append(data,0xa9,0x05,0x9c,0xbb);data=append(data,make([]byte,12)...);data=append(data,addressBytes...);data=append(data,pad32(amount)...);return data,nil
}

type rpcError struct{Code int;Message string}
type rpcResponse struct{JSONRPC string;ID int;Result json.RawMessage;Error *rpcError}

func rpcCall(method string,params interface{})(json.RawMessage,error){
    body,err:=json.Marshal(map[string]interface{}{"jsonrpc":"2.0","id":1,"method":method,"params":params});if err!=nil{return nil,err}
    req,err:=http.NewRequest(http.MethodPost,bscRPCURL,bytes.NewReader(body));if err!=nil{return nil,err};req.Header.Set("Content-Type","application/json")
    resp,err:=http.DefaultClient.Do(req);if err!=nil{return nil,fmt.Errorf("BSC RPC request failed: %w",err)};defer resp.Body.Close()
    raw,err:=io.ReadAll(resp.Body);if err!=nil{return nil,err};if resp.StatusCode<200||resp.StatusCode>=300{return nil,fmt.Errorf("BSC RPC returned HTTP %d",resp.StatusCode)}
    var decoded rpcResponse;if err:=json.Unmarshal(raw,&decoded);err!=nil{return nil,fmt.Errorf("invalid BSC RPC response: %w",err)};if decoded.Error!=nil{return nil,fmt.Errorf("BSC RPC error %d: %s",decoded.Error.Code,decoded.Error.Message)}
    return decoded.Result,nil
}

func hexQuantityToBig(raw json.RawMessage)(*big.Int,error){
    var s string;if err:=json.Unmarshal(raw,&s);err!=nil{return nil,err};s=strings.TrimPrefix(s,"0x");if s==""{return big.NewInt(0),nil}
    n:=new(big.Int);if _,ok:=n.SetString(s,16);!ok{return nil,fmt.Errorf("invalid hex quantity")};return n,nil
}

func isHexAddress(s string) bool {
    s = strings.TrimPrefix(strings.TrimSpace(s), "0x")
    if len(s) != 40 { return false }
    _, err := hex.DecodeString(s)
    return err == nil
}

func getEnv(key string) string { return os.Getenv(key) }

func getTreasuryDiagnostics() (string, *big.Int, *big.Int, error) {
    privateKeyHex := strings.TrimSpace(getEnv("NIST_TREASURY_PRIVATE_KEY"))
    if privateKeyHex == "" {
        return "", nil, nil, fmt.Errorf("ยังไม่ได้ตั้งค่า NIST_TREASURY_PRIVATE_KEY บน Backend")
    }
    d, err := parsePrivateKey(privateKeyHex)
    if err != nil { return "", nil, nil, err }
    derived := ethAddressFromPrivate(d)

    bnbRaw, err := rpcCall("eth_getBalance", []interface{}{derived, "latest"})
    if err != nil { return derived, nil, nil, fmt.Errorf("อ่าน tBNB balance ไม่สำเร็จ: %w", err) }
    bnb, err := hexQuantityToBig(bnbRaw)
    if err != nil { return derived, nil, nil, err }

    tokenCall := "0x70a08231" + strings.Repeat("0", 24) + strings.TrimPrefix(derived, "0x")
    tokenRaw, err := rpcCall("eth_call", []interface{}{map[string]string{
        "to": NISTContractAddress,
        "data": tokenCall,
    }, "latest"})
    if err != nil { return derived, bnb, nil, fmt.Errorf("อ่าน NIST balance ไม่สำเร็จ: %w", err) }
    token, err := hexQuantityToBig(tokenRaw)
    if err != nil { return derived, bnb, nil, err }

    return derived, bnb, token, nil
}

func formatNIST(raw *big.Int) string {
    if raw == nil { return "unknown" }
    whole := new(big.Int).Quo(new(big.Int).Set(raw), big.NewInt(1000000000000000000))
    return whole.String()
}

func formatBNB(raw *big.Int) string {
    if raw == nil { return "unknown" }
    whole := new(big.Int).Quo(new(big.Int).Set(raw), big.NewInt(1000000000000000000))
    frac := new(big.Int).Mod(new(big.Int).Set(raw), big.NewInt(1000000000000000000))
    return fmt.Sprintf("%s.%018s", whole.String(), frac.String())
}

func sendNISTTransfer(toWallet string,amountNIST int64)(string,error){
    privateKeyHex:=strings.TrimSpace(getEnv("NIST_TREASURY_PRIVATE_KEY"));if privateKeyHex==""{return "",fmt.Errorf("ยังไม่ได้ตั้งค่า NIST_TREASURY_PRIVATE_KEY บน Backend")}
    d,err:=parsePrivateKey(privateKeyHex);if err!=nil{return "",err}
    derived:=ethAddressFromPrivate(d);if !strings.EqualFold(derived,ReceiverAddress){return "",fmt.Errorf("private key ของ Treasury ไม่ตรงกับ RECEIVER_ADDRESS")}
    amount:=new(big.Int).Mul(big.NewInt(amountNIST),new(big.Int).Exp(big.NewInt(10),big.NewInt(18),nil))
    data,err:=buildTransferData(toWallet,amount);if err!=nil{return "",err}
    nonceRaw,err:=rpcCall("eth_getTransactionCount",[]interface{}{derived,"pending"});if err!=nil{return "",fmt.Errorf("อ่าน nonce ไม่สำเร็จ: %w",err)}
    nonce,err:=hexQuantityToBig(nonceRaw);if err!=nil{return "",err}
    gasPriceRaw,err:=rpcCall("eth_gasPrice",[]interface{}{});if err!=nil{return "",fmt.Errorf("อ่าน gas price ไม่สำเร็จ: %w",err)}
    gasPrice,err:=hexQuantityToBig(gasPriceRaw);if err!=nil{return "",err}
    gasLimit:=big.NewInt(100000);to:=mustBig("EC08895F6C21f17b9b4D32b5bE3CcAA79b0E58ED")
    unsigned:=rlpList(rlpUint(nonce),rlpUint(gasPrice),rlpUint(gasLimit),rlpBytes(to.Bytes()),rlpUint(big.NewInt(0)),rlpBytes(data),rlpUint(big.NewInt(BSCChainID)),rlpUint(big.NewInt(0)),rlpUint(big.NewInt(0)))
    digest:=keccak256(unsigned);r,s,recovery,err:=signDigest(d,digest);if err!=nil{return "",fmt.Errorf("เซ็น transaction ไม่สำเร็จ: %w",err)}
    v:=new(big.Int).Add(big.NewInt(int64(recovery)),big.NewInt(35));v.Add(v,new(big.Int).Mul(big.NewInt(2),big.NewInt(BSCChainID)))
    signed:=rlpList(rlpUint(nonce),rlpUint(gasPrice),rlpUint(gasLimit),rlpBytes(to.Bytes()),rlpUint(big.NewInt(0)),rlpBytes(data),rlpUint(v),rlpUint(r),rlpUint(s))
    result,err:=rpcCall("eth_sendRawTransaction",[]interface{}{"0x"+hex.EncodeToString(signed)});if err!=nil{return "",fmt.Errorf("ส่ง transaction ไม่สำเร็จ: %w",err)}
    var txHash string;if err:=json.Unmarshal(result,&txHash);err!=nil||txHash==""{return "",fmt.Errorf("BSC RPC ไม่ส่ง tx hash กลับมา")};return txHash,nil
}
