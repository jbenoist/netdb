# Network Database Library

## What is this ?
`netdb` is a small library allowing to map objects to IPv6/IPv4 networks. Given
an IPv4/IPv6 address, it may be queried to assess membership of a host to a
known network, and optionally to efficiently retrieve the details of the closest
block.

## Implementation details
### Algorithm
The underlying data-structure is a binary radix tree, which provides rather
descent space complexity while guaranteeing fast lookups and inserts.
### API notes
* There is currently no `delete` operation.
* The `add` operation takes either an IPv4 or IPv6 *host*, or an IPv4 or IPv6
  *network* in CIDR notation.
* The `lookup` operation takes either an IPv4 or IPv6 *host*.
* When querying the database with a host that is contained by overlapping
  networks, the closest network is returned.
* If a network that already exists is added, the existing object is silently
  overwritten with the new one.

## Using netdb
```
import "netdb"

db := &netdb.NetDB{}

err := db.Add("224.0.0.0/4", "multicast")
db.Add("224.0.0.251", "mDNS")
db.Add("::1", "loopback")

fmt.Printf("%v networks across %v vertices\n", db.Networks(), db.Nodes())
/* -> '3 hosts across 4 vertices' */

ip, mask, data, err := db.Lookup("224.0.0.251")
fmt.Printf("%v/%v={%v}\n", ip, mask, data)
/* -> '224.0.0.251/32={mDNS}' */

ip, mask, data, err = db.Lookup("224.0.0.1")
fmt.Printf("%v/%v={%v}\n", ip, mask, data)
/* -> '224.0.0.0/4={multicast}' */

_, _, _, err = db.Lookup("194.2.218.254")
/* err != nil */
```

## Example
see `netdb_test.go`
