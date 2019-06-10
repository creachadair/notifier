// Program userkey generates a passphrase using keyfish.
//
// Usage:
//    userkey -server :8080 [-print] <hostname>
//
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"bitbucket.org/creachadair/jrpc2"
	"github.com/creachadair/keyfish/config"
	"github.com/creachadair/notifier"
)

var (
	doList  = flag.Bool("list", false, "List known site names")
	doPrint = flag.Bool("print", false, "Print the result instead of copying it")
	doShow  = flag.Bool("show", false, "Show the configuration for a site")
	doFull  = flag.Bool("full", false, "Show the full configuration for a site (implies -show)")
	doLax   = flag.Bool("lax", false, "Accept site names that do not match known configurations")
	doRaw   = flag.Bool("raw", false, "Print results as raw JSON")
)

func generateKey(ctx context.Context, cli *jrpc2.Client, req *notifier.KeyGenRequest) (result *notifier.KeyGenReply, err error) {
	err = cli.CallResult(ctx, "Key.Generate", req, &result)
	return
}

func listSites(ctx context.Context, cli *jrpc2.Client) (result []string, err error) {
	err = cli.CallResult(ctx, "Key.List", nil, &result)
	return
}

func showSite(ctx context.Context, cli *jrpc2.Client, req *notifier.SiteRequest) (result *config.Site, err error) {
	err = cli.CallResult(ctx, "Key.Site", req, &result)
	return
}

func init() { notifier.RegisterFlags() }

func main() {
	flag.Parse()
	if flag.NArg() == 0 && !*doList {
		log.Fatal("You must provide a hostname or salt@hostname")
	}
	ctx, cli, err := notifier.Dial(context.Background())
	if err != nil {
		log.Fatalf("Dial: %v", err)
	}
	defer cli.Close()

	if *doList {
		sites, err := listSites(ctx, cli)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(strings.Join(sites, "\n"))
		return
	}

	site, salt := parseHost(flag.Arg(0))
	if *doShow || *doFull {
		site, err := showSite(ctx, cli, &notifier.SiteRequest{
			Host:   site,
			Full:   *doFull,
			Strict: !*doLax,
		})
		if err != nil {
			log.Fatal(err)
		}
		bits, err := json.Marshal(site)
		if err != nil {
			log.Fatalf("Marshaling JSON: %v", err)
		}
		if *doFull {
			var buf bytes.Buffer
			json.Indent(&buf, bits, "", "  ")
			bits = buf.Bytes()
		}
		fmt.Println(string(bits))
		return
	}

	pw, err := generateKey(ctx, cli, &notifier.KeyGenRequest{
		Host:   site,
		Salt:   salt,
		Copy:   !*doPrint,
		Strict: !*doLax,
	})
	if e, ok := err.(*jrpc2.Error); ok && e.Code() == notifier.UserCancelled {
		os.Exit(2)
	} else if err != nil {
		log.Fatal(err)
	}
	if *doRaw {
		bits, err := json.Marshal(pw)
		if err != nil {
			log.Fatalf("Marshaling response: %v", err)
		}
		fmt.Println(string(bits))
	} else if pw.Key == "" {
		fmt.Print(pw.Label, "\t", pw.Hash, "\n")
	} else {
		fmt.Println(pw.Key)
	}
}

func parseHost(host string) (site string, salt *string) {
	parts := strings.SplitN(host, "@", 2)
	if len(parts) == 2 {
		return parts[1], &parts[0]
	}
	return parts[0], nil
}
