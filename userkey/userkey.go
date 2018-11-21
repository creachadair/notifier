// Program userkey generates a passphrase using keyfish.
//
// Usage:
//    userkey -server :8080 [-print] <hostname>
//
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/jrpc2/caller"
	"bitbucket.org/creachadair/jrpc2/channel"
	"bitbucket.org/creachadair/keyfish/config"
	"bitbucket.org/creachadair/notifier"
)

var (
	serverAddr = flag.String("server", os.Getenv("NOTIFIER_ADDR"), "Server address")
	doList     = flag.Bool("list", false, "List known site names")
	doPrint    = flag.Bool("print", false, "Print the result instead of copying it")
	doShow     = flag.Bool("show", false, "Show the configuration for a site")
	doFull     = flag.Bool("full", false, "Show the full configuration for a site (implies -show)")

	generateKey = caller.New("Key.Generate", caller.Options{
		Params: (*notifier.KeyGenRequest)(nil),
		Result: (*notifier.KeyGenReply)(nil),
	}).(func(context.Context, *jrpc2.Client, *notifier.KeyGenRequest) (*notifier.KeyGenReply, error))
	listSites = caller.New("Key.List", caller.Options{
		Params: nil,
		Result: []string(nil),
	}).(func(context.Context, *jrpc2.Client) ([]string, error))
	showSite = caller.New("Key.Site", caller.Options{
		Params: (*notifier.SiteRequest)(nil),
		Result: (*config.Site)(nil),
	}).(func(context.Context, *jrpc2.Client, *notifier.SiteRequest) (*config.Site, error))
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
	cli := jrpc2.NewClient(channel.RawJSON(conn, conn), nil)
	defer cli.Close()
	ctx := context.Background()

	if *doList {
		sites, err := listSites(ctx, cli)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(strings.Join(sites, "\n"))
		return
	}
	if *doShow || *doFull {
		site, err := showSite(ctx, cli, &notifier.SiteRequest{
			Host: flag.Arg(0),
			Full: *doFull,
		})
		if err != nil {
			log.Fatal(err)
		}
		bits, err := json.Marshal(site)
		if err != nil {
			log.Fatalf("Marshaling JSON: %v", err)
		}
		fmt.Println(string(bits))
		return
	}
	pw, err := generateKey(ctx, cli, &notifier.KeyGenRequest{
		Host: flag.Arg(0),
		Copy: !*doPrint,
	})
	if e, ok := err.(*jrpc2.Error); ok && e.Code() == notifier.UserCancelled {
		os.Exit(2)
	} else if err != nil {
		log.Fatal(err)
	}
	if pw.Key == "" {
		fmt.Print(pw.Label, "\t", pw.Hash, "\n")
	} else {
		fmt.Println(pw.Key)
	}
}
