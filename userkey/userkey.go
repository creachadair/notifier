// Program userkey generates a passphrase using keyfish.
//
// Usage:
//    userkey -server :8080 [-print] <hostname>
//
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/jrpc2/channel"
	"bitbucket.org/creachadair/misctools/notifier"
)

var (
	serverAddr = flag.String("server", os.Getenv("NOTIFIER_ADDR"), "Server address")
	doList     = flag.Bool("list", false, "List known site names")
	doPrint    = flag.Bool("print", false, "Print the result instead of copying it")

	generateKey = jrpc2.NewCaller("Key.Generate", (*notifier.KeyGenRequest)(nil), "").(func(*jrpc2.Client, *notifier.KeyGenRequest) (string, error))
	listSites   = jrpc2.NewCaller("Key.List", nil, []string(nil)).(func(*jrpc2.Client) ([]string, error))
)

func main() {
	flag.Parse()
	if flag.NArg() == 0 && !*doList {
		log.Fatal("You must provide a hostname")
	}
	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Dial %q: %v", *serverAddr, err)
	}
	cli := jrpc2.NewClient(channel.Raw(conn), nil)
	defer cli.Close()

	if *doList {
		sites, err := listSites(cli)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(strings.Join(sites, "\n"))
		return
	}
	pw, err := generateKey(cli, &notifier.KeyGenRequest{
		Host: flag.Arg(0),
		Copy: !*doPrint,
	})
	if e, ok := err.(*jrpc2.Error); ok && e.Code == notifier.E_UserCancelled {
		os.Exit(2)
	} else if err != nil {
		log.Fatal(err)
	}
	fmt.Println(pw)
}
