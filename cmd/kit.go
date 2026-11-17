package cmd

import (
	"os/exec"
	"strings"

	"github.com/FacileStudio/facile/internal/ui"
)

const kitInstallHint = "curl -fsSL https://get.facile.studio | sh"

// kitCommand maps a facile package command, aliases included, to its kit
// name. Commands kit does not own, login and doctor among them, map to "".
func kitCommand(name string) string {
	switch name {
	case "install", "add":
		return "add"
	case "uninstall", "remove", "rm", "del":
		return "del"
	case "update", "list":
		return name
	}
	return ""
}

// kitArgs finds the subcommand in a facile argument list and rewrites it for
// kit, keeping every other argument, flags included, in its original order.
// It reports false when the command stays in facile: an unmapped command, or
// an update that names facile itself, which is facile's own self-update.
func kitArgs(args []string) (string, []string, bool) {
	at := subcommandIndex(args)
	if at < 0 {
		return "", nil, false
	}
	mapped := kitCommand(args[at])
	if mapped == "" || (mapped == "update" && namesSelf(args[at+1:])) {
		return "", nil, false
	}
	out := append([]string{mapped}, args[:at]...)
	return args[at], append(out, args[at+1:]...), true
}

// subcommandIndex skips the root's persistent flags to find the first
// positional argument. --bin-dir is the one root flag that takes a value.
func subcommandIndex(args []string) int {
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--":
			return -1
		case arg == "--bin-dir":
			i++
		case !strings.HasPrefix(arg, "-"):
			return i
		}
	}
	return -1
}

func namesSelf(args []string) bool {
	for _, name := range splitNames(args) {
		if strings.EqualFold(name, selfTool().Name) {
			return true
		}
	}
	return false
}

// forwardToKit hands a package command to kit when kit is installed, and
// otherwise says how to get it and lets facile run the command as before.
func forwardToKit(args []string, lookPath func(string) (string, error)) {
	from, rest, ok := kitArgs(args)
	if !ok {
		return
	}
	path, err := lookPath("kit")
	if err != nil {
		ui.Warn("facile %s is moving to kit, get it with `%s`", from, kitInstallHint)
		return
	}
	ui.Warn("facile %s is now kit %s", from, rest[0])
	runKit(path, rest)
}

// lookPathKit is the production lookup for forwardToKit.
func lookPathKit(name string) (string, error) { return exec.LookPath(name) }
