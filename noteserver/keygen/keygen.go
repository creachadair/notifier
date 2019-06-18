// Package keygen implements a key generator service based on keyfish.
package keygen

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"bitbucket.org/creachadair/stringset"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/keyfish/config"
	"github.com/creachadair/keyfish/wordhash"
	"github.com/creachadair/notifier"
)

func init() { notifier.RegisterPlugin("Key", new(keygen)) }

type keygen struct {
	noteconf *notifier.Config

	μ   sync.Mutex
	cfg *config.Config
}

// Init implements a method of notifier.Plugin.
func (k *keygen) Init(cfg *notifier.Config) error {
	if cfg.Key.ConfigFile == "" {
		return notifier.ErrNotApplicable
	}
	k.noteconf = cfg
	return k.Update()
}

// Update implements a method of notifier.Plugin.
func (k *keygen) Update() error {
	// Reload the configuration file.
	path := os.ExpandEnv(k.noteconf.Key.ConfigFile)
	nc, err := loadKeyConfig(path)
	if err != nil {
		return fmt.Errorf("loading key generator config: %v", err)
	}
	log.Printf("Loaded config from %q", path)
	k.μ.Lock()
	k.cfg = nc
	k.μ.Unlock()
	return nil
}

// Assigner implements a method of notifier.Plugin.
func (k *keygen) Assigner() jrpc2.Assigner { return handler.NewService(k) }

func (k *keygen) site(host string) (config.Site, bool) {
	k.μ.Lock()
	defer k.μ.Unlock()
	return k.cfg.Site(host)
}

func mergeSiteReq(site *config.Site, req *notifier.KeyGenRequest) {
	if req.Format != nil {
		site.Format = *req.Format
	}
	if req.Length != nil {
		site.Length = *req.Length
	}
	if req.Punct != nil {
		site.Punct = req.Punct
	}
	if req.Salt != nil {
		site.Salt = *req.Salt
	}
}

// Generate generates a passphrase for the given request.
func (k *keygen) Generate(ctx context.Context, req *notifier.KeyGenRequest) (*notifier.KeyGenReply, error) {
	if req.Host == "" {
		return nil, jrpc2.Errorf(code.InvalidParams, "missing host name")
	}
	const minLength = 6
	site, ok := k.site(req.Host)
	if !ok && req.Strict {
		return nil, jrpc2.Errorf(code.InvalidParams, "no match for host: %q", req.Host)
	}
	mergeSiteReq(&site, req)
	if site.Length < minLength {
		return nil, jrpc2.Errorf(code.InvalidParams, "invalid key length %d < %d", site.Length, minLength)
	} else if site.Format != "" && len(site.Format) < minLength {
		return nil, jrpc2.Errorf(code.InvalidParams, "invalid format length %d < %d", len(site.Format), minLength)
	}

	secret, err := notifier.PromptForText(ctx, &notifier.TextRequest{
		Prompt: fmt.Sprintf("Secret key for %q", site.Host),
		Hide:   true,
	})
	if err != nil {
		return nil, err
	}
	pctx := site.Context(secret)
	var pw string
	if fmt := site.Format; fmt != "" {
		pw = pctx.Format(site.Host, fmt)
	} else {
		pw = pctx.Password(site.Host, site.Length)
	}

	rsp := &notifier.KeyGenReply{
		Key:   pw,
		Hash:  wordhash.String(pw),
		Label: site.Host,
	}
	// If the caller asked us to copy to the clipboard, don't include the
	// passphrase in the response message.
	if req.Copy {
		notifier.SetSystemClipboard(ctx, []byte(pw))
		rsp.Key = ""
	}
	return rsp, nil
}

// List returns the names of the known configuration settings, in lexicographic order.
func (k *keygen) List(ctx context.Context) ([]string, error) {
	k.μ.Lock()
	defer k.μ.Unlock()
	sites := stringset.FromKeys(k.cfg.Sites)
	return sites.Elements(), nil
}

// Site returns the configuration settings for the specified site.
func (k *keygen) Site(ctx context.Context, req *notifier.SiteRequest) (*config.Site, error) {
	if req.Host == "" {
		return nil, jrpc2.Errorf(code.InvalidParams, "missing host name")
	}
	site, ok := k.site(req.Host)
	if !ok && req.Strict {
		return nil, jrpc2.Errorf(notifier.ResourceNotFound, "no config for %q", req.Host)
	}
	if !req.Full {
		site.Hints = nil
		site.OTP = nil
	}
	return &site, nil
}

func loadKeyConfig(path string) (*config.Config, error) {
	cfg := new(config.Config)
	if path == "" {
		cfg.Default.Length = 16
		return cfg, nil
	}
	if err := cfg.Load(path); err != nil {
		return nil, fmt.Errorf("loading key config: %v", err)
	}
	return cfg, nil
}
