package authflow

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/mattn/go-isatty"
	"golang.org/x/term"

	"github.com/FacileStudio/facile/internal/ui"
)

func interactive() bool { return isatty.IsTerminal(os.Stdin.Fd()) }

func ask(label string) (string, error) {
	if !interactive() {
		return "", fmt.Errorf("%s is needed and there is no terminal to ask on — pass it as a flag", strings.ToLower(label))
	}
	ui.Prompt("%s: ", label)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("cannot read your answer — run the command in a terminal")
	}
	return strings.TrimSpace(line), nil
}

// askSecret never echoes, and never returns the value anywhere but to its
// caller: no log line, no error message, no debug output.
func askSecret(label string) (string, error) {
	if !interactive() {
		return "", fmt.Errorf("a credential is needed and there is no terminal to ask on — run the command in a terminal")
	}
	ui.Prompt("%s: ", label)
	raw, err := term.ReadPassword(int(os.Stdin.Fd()))
	ui.Prompt("\n")
	if err != nil {
		return "", fmt.Errorf("cannot read the credential — run the command in a terminal")
	}
	return string(raw), nil
}

func confirm(question string) bool {
	if !interactive() {
		return true
	}
	ui.Prompt("%s [Y/n] ", question)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "" || answer == "y" || answer == "yes"
}

func openBrowser(url string) bool {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", url)
	case "windows":
		command = exec.Command("cmd", "/C", "start", "", url)
	default:
		command = exec.Command("xdg-open", url)
	}
	return command.Start() == nil
}

func envOf(name string) string {
	if name == "" {
		return ""
	}
	return os.Getenv(name)
}
