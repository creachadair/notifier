package notifier

import (
	"log"

	"bitbucket.org/creachadair/jrpc2"
)

// A Plugin exposes a set of methods.
type Plugin interface {
	// Init is called once before any other methods of the plugin are used, with
	// a pointer to the shared configuration.
	Init(*Config) error

	// Update may be called periodically to give the plugin an opportunity to
	// update its state.
	Update() error

	// Assigner returns an assigner for handlers.
	Assigner() jrpc2.Assigner
}

var plugins = make(map[string]Plugin)

// RegisterPlugin registers a plugin. This function will panic if the same name
// is registered multiple times.
func RegisterPlugin(name string, p Plugin) {
	if old, ok := plugins[name]; ok {
		log.Panicf("Duplicate registration for plugin %q: %v, %v", name, old, p)
	} else if p == nil {
		log.Panicf("Invalid nil plugin for %q", name)
	}
	plugins[name] = p
}

// PluginAssigner returns a jrpc2.Assigner that exports the methods of all the
// registered plugins.
func PluginAssigner(cfg *Config) jrpc2.Assigner {
	svc := make(jrpc2.ServiceMapper)
	for name, plug := range plugins {
		if err := plug.Init(cfg); err != nil {
			log.Panicf("Initializing plugin %q: %v", name, err)
		}
		svc[name] = plug.Assigner()
	}
	return svc
}
