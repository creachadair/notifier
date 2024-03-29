// Package poster implements a service that posts notifications to the user.
package poster

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/notifier"
)

func init() { notifier.RegisterPlugin("Notify", new(poster)) }

type poster struct {
	cfg *notifier.Config
}

// Init implements part of notifier.Plugin.
func (p *poster) Init(cfg *notifier.Config) error {
	p.cfg = cfg
	return nil
}

// Update implements part of notifier.Plugin. This implementation does nothing.
func (*poster) Update() error { return nil }

// Assigner implements part of notifier.Plugin.
func (p *poster) Assigner() handler.Map {
	return handler.Map{
		"Post": handler.New(p.Post),
		"Say":  handler.New(p.Say),
	}
}

// Post posts a textual notification to the user.
func (p *poster) Post(ctx context.Context, req *notifier.PostRequest) (bool, error) {
	if req.Body == "" && req.Title == "" {
		return false, jrpc2.Errorf(jrpc2.InvalidParams, "missing notification body and title")
	}
	program := []string{
		fmt.Sprintf("display notification %q", req.Body),
		fmt.Sprintf("with title %q", req.Title),
	}
	if t := req.Subtitle; t != "" {
		program = append(program, fmt.Sprintf("subtitle %q", t))
	}
	if req.Audible {
		program = append(program, fmt.Sprintf("sound name %q", p.cfg.Notify.Sound))
	}
	cmd := exec.CommandContext(ctx, "osascript")
	cmd.Stdin = strings.NewReader(strings.Join(program, " "))
	if wait := req.After; wait > 0 {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(req.After):
		}
	}
	err := cmd.Run()
	return err == nil, err
}

// Say delivers a voice notification to the user.
func (p *poster) Say(ctx context.Context, req *notifier.SayRequest) (bool, error) {
	if req.Text == "" {
		return false, jrpc2.Errorf(jrpc2.InvalidParams, "empty text")
	} else if req.Voice == "" {
		req.Voice = p.cfg.Notify.Voice
	}
	if wait := req.After; wait > 0 {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(wait):
		}
	}
	cmd := exec.CommandContext(ctx, "say", "-v", req.Voice)
	cmd.Stdin = strings.NewReader(req.Text)
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err == nil, err
}
