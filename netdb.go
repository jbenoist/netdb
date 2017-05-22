package netdb

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"strings"
)

type node struct {
	bits   []uint8
	mask   int
	parent *node
	zero   *node
	one    *node
	data   interface{}
}

type NetDB struct {
	root *node
}

func new_node(bits []uint8, ones int, parent, zero, one *node, data interface{}) *node {
	n := &node{bits, ones, parent, zero, one, data}
	if n.zero != nil {
		n.zero.parent = n
	}
	if n.one != nil {
		n.one.parent = n
	}
	return n
}

func to_array(ip net.IP, ones int) []uint8 {
	array := make([]uint8, 0)
	for i := 127; i >= 128-ones; i-- {
		bitval := uint8(0)
		if (ip[15-i/8] & (1 << uint(i%8))) != 0 {
			bitval = 1
		}
		array = append(array, bitval)
	}
	return array
}

func parse_cidr(netcidr string) (error, []uint8, int) {
	if !strings.Contains(netcidr, "/") {
		if strings.Contains(netcidr, ":") {
			netcidr += "/128"
		} else {
			netcidr += "/32"
		}
	}
	_, ipnet, err := net.ParseCIDR(netcidr)
	if err != nil {
		return err, nil, -1
	}
	ones, _ := ipnet.Mask.Size()
	if ones == 0 {
		return errors.New("incorrect parameter"), nil, -1
	}
	if len(ipnet.IP) == 4 {
		ones += 96
	}
	bits := to_array(ipnet.IP.To16(), ones)
	return nil, bits, ones
}

func (n *node) to16() []byte {
	array := make([]byte, 16)
	curbit := n.mask - 1
	for n != nil {
		for idx, _ := range n.bits {
			byte_idx := curbit / 8
			bit_idx := curbit % 8
			if n.bits[len(n.bits)-idx-1] != 0 {
				array[byte_idx] |= 1 << (7 - uint(bit_idx))
			}
			curbit--
		}
		n = n.parent
	}
	return array
}

func (n *node) adjmask() int {
	if IsIPv4Mapped(net.IP(n.to16())) {
		return n.mask - 96
	}
	return n.mask
}

func (db *NetDB) Add(netcidr string, data interface{}) error {
	err, bits, ones := parse_cidr(netcidr)
	if err != nil {
		return err
	}
	if db.root == nil {
		db.root = new_node(bits, ones, nil, nil, nil, data)
	} else {
		curnode_idx := 0
		curnode := db.root
		for bits_idx := 0; curnode != nil && bits_idx < len(bits); {
			if curnode_idx == len(curnode.bits) {
				childptr := &curnode.zero
				if bits[bits_idx] == 1 {
					childptr = &curnode.one
				}
				if *childptr == nil {
					*childptr = &node{bits[bits_idx:len(bits)], ones, curnode, nil, nil, data}
					break
				}
				curnode_idx = 0
				curnode = *childptr
			} else {
				if bits[bits_idx] != curnode.bits[curnode_idx] {
					newnode := new_node(bits[bits_idx:len(bits)], ones, curnode, nil, nil, data)
					oldnode := new_node(curnode.bits[curnode_idx:len(curnode.bits)], curnode.mask, curnode, curnode.zero, curnode.one, curnode.data)
					curnode.bits = curnode.bits[0:curnode_idx]
					curnode.mask = -1
					curnode.one, curnode.zero = oldnode, newnode
					if newnode.bits[0] == 1 {
						curnode.one, curnode.zero = newnode, oldnode
					}
					break
				} else if bits_idx+1 == len(bits) {
					if curnode_idx+1 == len(curnode.bits) {
						curnode.mask = ones
						curnode.data = data
					} else {
						oldnode := new_node(curnode.bits[curnode_idx+1:len(curnode.bits)], curnode.mask, curnode, curnode.zero, curnode.one, curnode.data)
						curnode.bits = curnode.bits[0 : curnode_idx+1]
						curnode.mask = ones
						curnode.data = data
						curnode.one, curnode.zero = nil, oldnode
						if oldnode.bits[0] == 1 {
							curnode.one, curnode.zero = oldnode, nil
						}
					}
				}
				curnode_idx++
				bits_idx++
			}
		}
	}
	return nil
}

func (db *NetDB) Remove(netcidr string) error {
	err, bits, ones := parse_cidr(netcidr)
	fmt.Println(err, bits, ones)
	return err
}

func IsIPv4Mapped(ip net.IP) bool {
	prefix := make([]byte, 12)
	prefix[10] = 0xff
	prefix[11] = 0xff
	return reflect.DeepEqual([]byte(ip[0:12]), prefix)
}

func (db *NetDB) Lookup(ipstr string) (net.IP, int, interface{}, error) {
	var prev *node = nil
	ip := net.ParseIP(ipstr)
	if db.root == nil {
		return nil, 0, nil, errors.New("not found")
	}
	bits := to_array(ip, 128)
	curnode_idx := 0
	curnode := db.root
	for bits_idx := 0; curnode != nil && bits_idx < len(bits); {
		if curnode_idx == len(curnode.bits) {
			curnode_idx = 0
			prev = curnode
			if bits[bits_idx] == 1 {
				curnode = curnode.one
			} else {
				curnode = curnode.zero
			}
		} else {
			if bits[bits_idx] != curnode.bits[curnode_idx] {
				prev = curnode.parent
				break
			}
			prev = curnode
			curnode_idx++
			bits_idx++
		}
	}
	for prev != nil && prev.mask == -1 {
		prev = prev.parent
	}
	if prev == nil {
		return nil, 0, nil, errors.New("not found")
	}
	netmask := prev.mask
	network := net.IP(prev.to16())
	if IsIPv4Mapped(network) {
		netmask -= 96
	}
	return network, netmask, prev.data, nil
}

func (n *node) count(hostsonly bool) int {
	if n != nil {
		c := 1
		if hostsonly && n.mask == -1 {
			c = 0
		}
		return c + n.one.count(hostsonly) + n.zero.count(hostsonly)
	}
	return 0
}

func (db *NetDB) Networks() int {
	return db.root.count(true)
}

func (db *NetDB) Nodes() int {
	return db.root.count(false)
}

func draw_edge(w io.Writer, c, n *node) {
	if n != nil {
		if n.mask == -1 {
			draw_edge(w, c, n.parent)
		} else {
			fmt.Fprintf(w, "\tn%p->n%p;\n", n, c)
		}
	}
}

func draw_vertex(w io.Writer, n *node) {
	if n != nil {
		if n.mask != -1 {
			fmt.Fprintf(w, "\tn%p [label=\"%v/%v\"];\n", n, net.IP(n.to16()), n.adjmask())
			draw_edge(w, n, n.parent)
		}
		draw_vertex(w, n.zero)
		draw_vertex(w, n.one)
	}
}

func (db *NetDB) Graph(file string) error {
	os.Remove(file)
	f, err := os.Create(file)
	if err == nil {
		defer f.Close()
		w := bufio.NewWriter(f)
		fmt.Fprintln(w, "digraph G {")
		fmt.Fprintln(w, "\tratio=\"auto\";")
		fmt.Fprintln(w, "\tranksep=\"3\";")
		fmt.Fprintln(w, "\tnode [fontname=\"courrier\", fontsize=\"8\"];")
		draw_vertex(w, db.root)
		fmt.Fprintln(w, "}")
		w.Flush()
	}
	return err
}
