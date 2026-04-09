package command

import (
	"sort"
	"strings"
)

// Command represents a slash command.
type Command struct {
	Name        string
	Description string
	Handler     func(app *AppState, args string) (string, error)
}

// Registry holds registered commands.
type Registry struct {
	commands map[string]Command
}

// NewRegistry creates an empty command registry.
func NewRegistry() *Registry {
	return &Registry{commands: make(map[string]Command)}
}

// Register adds a command to the registry.
func (r *Registry) Register(cmd Command) {
	r.commands[cmd.Name] = cmd
}

// Get returns the command with the given name, or nil if not found.
func (r *Registry) Get(name string) *Command {
	cmd, ok := r.commands[name]
	if !ok {
		return nil
	}
	return &cmd
}

// All returns all registered commands sorted by name.
func (r *Registry) All() []Command {
	cmds := make([]Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		cmds = append(cmds, cmd)
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].Name < cmds[j].Name
	})
	return cmds
}

// Parse checks whether input is a slash command. If input starts with "/",
// it splits on the first space to extract the command name and arguments.
func Parse(input string) (cmdName, args string, isCommand bool) {
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, "/") {
		return "", "", false
	}

	// Remove the leading slash.
	trimmed = trimmed[1:]

	parts := strings.SplitN(trimmed, " ", 2)
	cmdName = parts[0]
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}
	return cmdName, args, true
}
