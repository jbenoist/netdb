package netdb

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"reflect"
	"testing"
)

func testAdd(t *testing.T, db *NetDB, ip string, data interface{}, err bool) {
	if err && db.Add(ip, data) == nil {
		t.FailNow()
	} else if !err && db.Add(ip, data) != nil {
		t.FailNow()
	}
}

func testLookup(t *testing.T, db *NetDB, key string, network net.IP, netmask int, data interface{}, err bool) {

	ip, mask, rdata, e := db.Lookup(key)
	if err != (e != nil) {
		t.FailNow()
	} else if !err {
		if mask < netmask {
			t.FailNow()
		} else if mask >= netmask {
			if !reflect.DeepEqual(ip, network) {
				subip, _, _ := net.ParseCIDR(fmt.Sprintf("%v/%v", ip, netmask))
				if !reflect.DeepEqual(subip, ip) {
					t.FailNow()
				}
			} else if mask == netmask && rdata.(string) != data.(string) {
				t.FailNow()
			}
		}
	}
}

func randomIP(ipv4 bool) net.IP {
	i := 0
	bits := make([]byte, 16)
	if ipv4 {
		for j := 0; j < 4; j++ {
			bits[j] = 0
		}
		i += 12
		bits[10] = 0xff
		bits[11] = 0xff
	}
	for ; i < 16; i += 4 {
		bits[i] = byte((rand.Uint32() & 0xff000000) >> 24)
		bits[i+1] = byte((rand.Uint32() & 0x00ff0000) >> 16)
		bits[i+2] = byte((rand.Uint32() & 0x0000ff00) >> 8)
		bits[i+3] = byte((rand.Uint32() & 0x000000ff))
	}
	return net.IP(bits)
}

func randomMask(ipv4 bool) uint {
	if ipv4 {
		return 1 + uint(rand.Uint32()%32)
	}
	return 1 + uint(rand.Uint32()%128)
}

func randomIpLocal(ipnet net.IP, mask uint) net.IP {
	bits := make([]byte, 16)
	copy(bits, ipnet)
	if IsIPv4Mapped(ipnet) {
		mask += 96
	}
	if mask == 0 {
		mask++
	}
	num_changes := uint(rand.Uint32() % uint32(mask+1))
	for i := uint(0); i < num_changes; i++ {
		idx := uint(0)
		mod := 128 - mask
		if mod != 0 {
			idx = uint(rand.Uint32()) % uint(mod)
			bits[15-idx/8] |= (1 << uint(idx%8))
		}
	}
	return net.IP(bits)
}

type testnet struct {
	ip   net.IP
	mask uint
	data interface{}
}

func TestAuto(t *testing.T) {
	db := &NetDB{}
	testcases := make([]testnet, 0)
	for i := 0; i < 100000; i++ {
		do_ipv4 := true
		if rand.Uint32()%2 != 0 {
			do_ipv4 = false
		}
		ip := randomIP(do_ipv4)
		mask := randomMask(do_ipv4)
		if IsIPv4Mapped(ip) != do_ipv4 {
			t.FailNow()
		}
		_, ipnet, err := net.ParseCIDR(fmt.Sprintf("%v/%v", ip, mask))
		if err != nil {
			t.FailNow()
		}
		key := fmt.Sprintf("%v/%v", ipnet.IP, mask)
		testcases = append(testcases, testnet{ipnet.IP.To16(), mask, key})
		testAdd(t, db, key, key, false)
	}
	for i := len(testcases) - 1; i > 0; i-- {
		j := rand.Int() % (i + 1)
		testcases[i], testcases[j] = testcases[j], testcases[i]
	}
	for _, test := range testcases {
		q_ip := randomIpLocal(test.ip, test.mask)
		testLookup(t, db, fmt.Sprintf("%v", q_ip), test.ip, int(test.mask), test.data, false)
	}
}

func TestEdgeCases(t *testing.T) {
	db := &NetDB{}
	if db.Add("127.0.0.1/0", "fail") == nil {
		t.FailNow()
	}
	if db.Add("194.2.218.254", "foo") != nil {
		t.FailNow()
	}
	if db.Add("::1", "bar") != nil {
		t.FailNow()
	}
	_, mask, data, _ := db.Lookup("::1")
	if mask != 128 || data != "bar" {
		t.FailNow()
	}
	_, mask, data, _ = db.Lookup("194.2.218.254")
	if mask != 32 || data != "foo" {
		t.FailNow()
	}
}

func TestNetworks(t *testing.T) {
	db := &NetDB{}
	for i := 0; i < 20; i++ {
		for j := 0; j < 20; j++ {
			for k := 0; k < 20; k++ {
				for l := 0; l < 20; l++ {
					key := fmt.Sprintf("%v.%v.%v.%v", i, j, k, l)
					db.Add(key, key)
				}
			}
		}
	}
	if db.Networks() != int(math.Pow(20, 4)) {
		t.FailNow()
	}
	db.Add("19.19.19.19", "19.19.19.19")
	if db.Networks() != int(math.Pow(20, 4)) {
		t.FailNow()
	}
	db.Add("::1", "::1")
	if db.Networks() == int(math.Pow(20, 4)) {
		t.FailNow()
	}
}
