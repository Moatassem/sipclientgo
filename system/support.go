package system

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"os"

	"net"
	"strings"
	"unicode/utf8"
)

// ============================================================

func GetLocalIPs() ([]net.IP, error) {
	var IPs []net.IP
	var ip net.IP
	ifaces, _ := net.Interfaces()
outer:
	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagRunning == 0 { //|| i.Flags&net.FlagBroadcast == 0
			continue
		}
		addrs, _ := i.Addrs()
		for _, addr := range addrs {
			if v, ok := addr.(*net.IPNet); ok {
				ip = v.IP
				if ip.To4() != nil && ip.IsPrivate() {
					IPs = append(IPs, ip)
					continue outer
				}
			}
		}
	}
	if len(IPs) == 0 {
		return nil, errors.New("no valid IPv4 found")
	}
	return IPs, nil
}

func GetLocalIPv4(getfirst bool) net.IP {
	fmt.Print("Checking Interfaces...")
	serverIPs, err := GetLocalIPs()
	if err != nil {
		fmt.Println("Failed to find an IPv4 interface:", err)
		os.Exit(1)
	}
	var serverIP net.IP
	if len(serverIPs) == 1 {
		serverIP = serverIPs[0]
		fmt.Println("Found (1):", serverIP)
	} else {
		var idx int
		for {
			fmt.Printf("Found (%d):\n", len(serverIPs))
			for i, s := range serverIPs {
				fmt.Printf("%d- %s\n", i+1, s.String())
			}
			if getfirst {
				idx = 1
				break
			} else {
				fmt.Print("Your choice:? ")
				n, err := fmt.Scanln(&idx)
				if n == 0 {
					log.Panic("no proper interface selected")
				}
				if idx <= 0 || idx > len(serverIPs) {
					fmt.Println("Invalid interface selected")
					continue
				}
				if err == nil {
					break
				}
				fmt.Println(err)
			}
		}
		serverIP = serverIPs[idx-1]
		fmt.Println("Selected:", serverIP)
	}
	return serverIP
}

func StartListening(ip net.IP, prt int) (*net.UDPConn, error) {
	if ip == nil {
		return nil, errors.New("nil IP address")
	}
	var socket net.UDPAddr
	socket.IP = ip
	socket.Port = prt
	return net.ListenUDP("udp", &socket)
}

func TestListening(ip net.IP, prt int) error {
	if ip == nil {
		return errors.New("nil IP address")
	}
	var socket net.UDPAddr
	socket.IP = ip
	socket.Port = prt
	conn, err := net.ListenUDP("udp", &socket)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

func GetUDPAddrFromConn(conn *net.UDPConn) *net.UDPAddr {
	return conn.LocalAddr().(*net.UDPAddr)
}

func GetUDPAddrStringFromConn(conn *net.UDPConn) string {
	return (conn.LocalAddr()).String()
}

func GetUDPortFromConn(conn *net.UDPConn) int {
	return conn.LocalAddr().(*net.UDPAddr).Port
}

func BuildUDPAddr(ip string, prt int) (*net.UDPAddr, error) {
	return net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", ip, prt))
}

func BuildUDPAddrFromSocketString(sckt string) (*net.UDPAddr, error) {
	return net.ResolveUDPAddr("udp", sckt)
}

func AreUAddrsEqual(addr1, addr2 *net.UDPAddr) bool {
	if addr1 == nil || addr2 == nil {
		return addr1 == addr2
	}
	return addr1.IP.Equal(addr2.IP) && addr1.Port == addr2.Port && addr1.Zone == addr2.Zone
}

func AreUDPConnsEqual(conn1, conn2 *net.UDPConn) bool {
	if conn1 == nil || conn2 == nil {
		return conn1 == conn2
	}

	addr1 := conn1.LocalAddr()
	addr2 := conn2.LocalAddr()

	return addr1.String() == addr2.String()
}

// ============================================================

func GenerateViaWithoutBranch(conn *net.UDPConn) string {
	udpsocket := GetUDPAddrFromConn(conn)
	return fmt.Sprintf("SIP/2.0/UDP %s", udpsocket)
}

func GenerateContact(skt *net.UDPAddr) string {
	return fmt.Sprintf("<sip:%s;transport=udp>", skt)
}

// =============================================================

func TrimWithSuffix(s string, sfx string) string {
	s = strings.Trim(s, " ")
	if s == "" {
		return s
	}
	return fmt.Sprintf("%s%s", s, sfx)
}

func GetNextIndex(pdu []byte, markstrng string) int {
	markBytes := []byte(markstrng)
	for i := 0; i <= len(pdu)-len(markBytes); i++ {
		k := 0
		for k < len(markBytes) {
			if pdu[i+k] != markBytes[k] {
				goto nextloop
			}
			k++
		}
		return i
	nextloop:
	}
	return -1
}

func GetUsedSize(pdu []byte) int {
	sz := len(pdu)
	for i := 0; i < sz; i++ {
		if pdu[i] == 0 {
			return i
		}
	}
	return sz
}

func DropVisualSeparators(strng string) string {
	var sb strings.Builder
	for _, r := range strng {
		switch r {
		case '.', '-', '(', ')':
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func KeepOnlyNumerics(strng string) string {
	var sb strings.Builder
	for _, r := range strng {
		if r < '0' || r > '9' {
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

func CleanAndSplitHeader(HeaderValue string, DropParameterValueDQ ...bool) map[string]string {
	if HeaderValue == "" {
		return nil
	}

	NVC := make(map[string]string)
	splitChar := ';'

	splitCharFirstIndex := strings.IndexRune(HeaderValue, splitChar)
	if splitCharFirstIndex == -1 {
		NVC["!headerValue"] = HeaderValue
		return NVC
	} else {
		NVC["!headerValue"] = HeaderValue[:splitCharFirstIndex]
	}

	chrlst := []rune(HeaderValue[splitCharFirstIndex:])
	var sb strings.Builder

	var fn, fv string
	DQO := false
	dropDQ := len(DropParameterValueDQ) > 0 && DropParameterValueDQ[0]

	for i := 0; i < len(chrlst); {
		switch chrlst[i] {
		case ' ':
			if DQO {
				sb.WriteRune(chrlst[i])
			}
		case '=':
			if DQO {
				sb.WriteRune(chrlst[i])
			} else {
				fn = sb.String()
				sb.Reset()
			}
		case splitChar:
			if DQO {
				sb.WriteRune(chrlst[i])
			} else {
				if sb.Len() == 0 {
					break
				}
				fv = sb.String()
				NVC[fn] = DropConcatenationChars(fv, dropDQ)
				fn, fv = "", ""
				sb.Reset()
			}
		case '"':
			if DQO {
				fv = sb.String()
				NVC[fn] = DropConcatenationChars(fv, dropDQ)
				fn, fv = "", ""
				sb.Reset()
				DQO = false
			} else {
				DQO = true
			}
		default:
			sb.WriteRune(chrlst[i])
		}
		chrlst = append(chrlst[:i], chrlst[i+1:]...)
	}

	if fn != "" && sb.Len() > 0 {
		fv = sb.String()
		NVC[fn] = DropConcatenationChars(fv, dropDQ)
	}

	return NVC
}

func DropConcatenationChars(s string, dropDQ bool) string {
	if dropDQ {
		s = strings.ReplaceAll(s, "'", "")
		return strings.ReplaceAll(s, `"`, "")
	}
	return s
}

func ParseParameters(parsline string) *map[string]string {
	parsline = strings.Trim(parsline, ";")
	parsline = strings.Trim(parsline, ",")
	parsMap := make(map[string]string)
	if parsline == "" {
		return &parsMap
	}
	for tpl := range strings.SplitSeq(parsline, ";") {
		tmp := strings.SplitN(tpl, "=", 2)
		switch len(tmp) {
		case 1:
			if _, ok := parsMap[tmp[0]]; !ok {
				parsMap[tmp[0]] = ""
			} else {
				LogError(LTSIPStack, fmt.Sprintf("duplicate parameter: [%s] - in line: [%s]", tmp[0], parsline))
			}
		case 2:
			if _, ok := parsMap[tmp[0]]; !ok {
				parsMap[tmp[0]] = tmp[1]
			} else {
				LogError(LTSIPStack, fmt.Sprintf("duplicate parameter: [%s] - in line: [%s]", tmp[0], parsline))
			}
		default:
			LogError(LTSIPStack, fmt.Sprintf("badly formatted parameter line: [%s]", parsline))
		}
	}
	return &parsMap
}

func GenerateParameters(pars *map[string]string) string {
	if pars == nil {
		return ""
	}
	var sb strings.Builder
	for k, v := range *pars {
		if v == "" {
			sb.WriteString(fmt.Sprintf(";%v", k))
		} else {
			sb.WriteString(fmt.Sprintf(";%v=%v", k, v))
		}
	}
	return sb.String()
}

func RandomNum(min, max uint32) uint32 {
	// #nosec G404: Ignoring gosec error - crypto is not required
	return rand.Uint32N(max-min+1) + min
}

// Convert string to int with default value with included minimum or maximum
func Str2IntDefaultMinMax[T int | int8 | int16 | int32 | int64](s string, d, min, max T) (T, bool) {
	out, ok := Str2IntCheck[T](s)
	if ok {
		if out < min || out > max {
			return d, false
		}
		return out, true
	}
	return d, false
}

func Str2IntCheck[T int | int8 | int16 | int32 | int64](s string) (T, bool) {
	var out T
	if len(s) == 0 {
		return out, false
	}
	idx := 0
	isN := s[idx] == '-'
	if isN {
		idx++
		if len(s) == 1 {
			return out, false
		}
	} else if s[idx] == '+' {
		idx++
	}
	for i := idx; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return out, false
		}
		out = out*10 + T(s[i]-'0')
	}
	if isN {
		out = -out
	}
	return out, true
}

func Str2Int[T int | int8 | int16 | int32 | int64](s string) T {
	var out T
	if len(s) == 0 {
		return out
	}
	idx := 0
	isN := s[idx] == '-'
	if isN {
		idx++
	}
	for i := idx; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return out
		}
		out = out*10 + T(s[i]-'0')
	}
	if isN {
		return -out
	}
	return out
}

func Str2Uint[T uint | uint8 | uint16 | uint32 | uint64](s string) T {
	var out T
	if len(s) == 0 {
		return out
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return out
		}
		out = out*10 + T(s[i]-'0')
	}
	return out
}

func Int2Str(val int) string {
	if val == 0 {
		return "0"
	}
	buf := make([]byte, 10)
	return int2str(buf, val)
}

func int2str[T int | int8 | int16 | int32 | int64](buf []byte, val T) string {
	isNeg := val < 0
	if isNeg {
		val *= -1
	}
	i := len(buf)
	for val >= 10 {
		i--
		buf[i] = '0' + byte(val%10)
		val /= 10
	}
	i--
	buf[i] = '0' + byte(val)

	if isNeg {
		return "-" + string(buf[i:])
	}
	return string(buf[i:])
}

func Uint16ToStr(val uint16) string {
	if val == 0 {
		return "0"
	}
	buf := make([]byte, 5)
	return uint2str(buf, val)
}

// Uint32ToStr converts a uint32 to its string representation.
func Uint32ToStr(val uint32) string {
	if val == 0 {
		return "0"
	}
	buf := make([]byte, 10)
	return uint2str(buf, val)
}

// Uint64ToStr converts a uint64 to its string representation.
func Uint64ToStr(val uint64) string {
	if val == 0 {
		return "0"
	}
	buf := make([]byte, 20)
	return uint2str(buf, val)
}

func uint2str[T uint16 | uint32 | uint64](buf []byte, val T) string {
	i := len(buf)
	for val >= 10 {
		i--
		buf[i] = '0' + byte(val%10)
		val /= 10
	}
	i--
	buf[i] = '0' + byte(val)

	return string(buf[i:])
}

//====================================================

func GetEnumString[T comparable](m map[T]string, s string, keepCase bool) T {
	if !keepCase {
		s = ASCIIToLower(s)
	}
	var rslt T
	for k, v := range m {
		if v == s {
			return k
		}
	}
	return rslt
}

//==================================================

func ASCIIToLower(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += byte(DeltaRune)
		}
		b.WriteByte(c)
	}
	return b.String()
}

func ASCIIToUpper(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'a' <= c && c <= 'z' {
			c -= byte(DeltaRune)
		}
		b.WriteByte(c)
	}
	return b.String()
}

func LowerDash(s string) string {
	return strings.ReplaceAll(ASCIIToLower(s), " ", "-")
}

func ASCIIPascal(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'a' <= c && c <= 'z' && (i == 0 || s[i-1] == '-') {
			c -= byte(DeltaRune)
		}
		b.WriteByte(c)
	}
	return b.String()
}

func ASCIIToLowerInPlace(s []byte) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		s[i] = c
	}
}

//==================================================

func Any[T any](items []*T, predict func(*T) bool) bool {
	for _, item := range items {
		if predict(item) {
			return true
		}
	}
	return false
}

func Find[T any](items []*T, predict func(*T) bool) *T {
	for _, item := range items {
		if predict(item) {
			return item
		}
	}
	return nil
}

func Filter[T any](items []*T, predict func(*T) bool) []*T {
	var result []*T
	for _, item := range items {
		if predict(item) {
			result = append(result, item)
		}
	}
	return result
}

func FirstKeyValue[T1 comparable, T2 any](m map[T1]T2) (T1, T2) {
	var key T1
	var value T2
	for k, v := range m {
		return k, v
	}
	return key, value
}

func Keys[T1 comparable, T2 any](m map[T1]T2) []T1 {
	var rslt []T1
	for k := range m {
		rslt = append(rslt, k)
	}
	return rslt
}

func FirstKey[T1 comparable, T2 any](m map[T1]T2) T1 {
	k, _ := FirstKeyValue(m)
	return k
}

func FirstValue[T1 comparable, T2 any](m map[T1]T2) T2 {
	_, v := FirstKeyValue(m)
	return v
}

func GetEnum[T1 comparable, T2 comparable](m map[T1]T2, i T2) T1 {
	var rslt T1
	for k, v := range m {
		if v == i {
			return k
		}
	}
	return rslt
}

func RemoveAt(slice []int, i int) []int {
	slice[i] = slice[len(slice)-1] // Move last element to index i
	return slice[:len(slice)-1]    // Trim the last element
}

// ===================================================================

func StringToHexString(input string) string {
	return BytesToHexString(StringToBytes(input))
}

func StringToBytes(input string) []byte {
	return []byte(input)
}

func BytesToHexString(data []byte) string {
	return hex.EncodeToString(data)
}

func BytesToBase64String(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func HashSDPBytes(bytes []byte) string {
	// hash := sha256.New()
	// return bytesToHexString(hash.Sum(bytes))
	hash := sha256.Sum256(bytes)
	return BytesToHexString(hash[:])
}

//===================================================================

func Stringlen(s string) int {
	return utf8.RuneCountInString(s)
}

// ====================================================

func IsProvisional(sc int) bool {
	return 100 <= sc && sc <= 199
}

func IsProvisional18x(sc int) bool {
	return 180 <= sc && sc <= 189
}

func Is18xOrPositive(sc int) bool {
	return (180 <= sc && sc <= 189) || (200 <= sc && sc <= 299)
}

func IsFinal(sc int) bool {
	return 200 <= sc && sc <= 699
}

func IsPositive(sc int) bool {
	return 200 <= sc && sc <= 299
}

func IsNegative(sc int) bool {
	return 300 <= sc && sc <= 699
}

func IsRedirection(sc int) bool {
	return 300 <= sc && sc <= 399
}

func IsNegativeClient(sc int) bool {
	return 400 <= sc && sc <= 499
}

func IsNegativeServer(sc int) bool {
	return 500 <= sc && sc <= 599
}

func IsNegativeGlobal(sc int) bool {
	return 600 <= sc && sc <= 699
}

//===================================================================
