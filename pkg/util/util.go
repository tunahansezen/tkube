package util

import (
	"errors"
	"fmt"
	"github.com/guumaster/logsymbols"
	"github.com/manifoldco/promptui"
	"net"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"
)

const (
	WaitSleep     = 5 * time.Second
	LocalDataPath = "$HOME/.tkube/data"
)

var (
	CommonValidator = func(input string) error {
		if len(input) < 1 {
			return errors.New("need some input")
		}
		match, _ := regexp.MatchString("^[a-zA-Z0-9_.-]*$", input)
		if !match {
			return errors.New("only use letters, numbers and dash")
		}
		return nil
	}
	ZeroTo255Validator = func(input string) error {
		if len(input) < 1 {
			return errors.New("need some input")
		}
		match, _ := regexp.MatchString("^[0-9]*$", input)
		if !match {
			return errors.New("only use numbers")
		}
		i, _ := strconv.Atoi(input)
		if i < 1 {
			return errors.New("input must be equal or greater than 0")
		}
		if i > 255 {
			return errors.New("input must be lower than 256")
		}
		return nil
	}
	IpValidator = func(input string) error {
		if len(input) < 1 {
			return errors.New("need some input")
		}
		ip := net.ParseIP(input)
		if ip == nil {
			return errors.New("not valid IP")
		}
		return nil
	}
	YesNoValidator = func(input string) error {
		li := strings.ToLower(input)
		if li != "y" && li != "n" && li != "yes" && li != "no" {
			return errors.New("enter y/Y or n/N")
		}
		return nil
	}
	PathValidator = func(input string) error {
		if len(input) < 1 {
			return errors.New("need some input")
		}
		pattern := `^(/[^/\0]+)+$`
		match, _ := regexp.MatchString(pattern, input)
		if !match {
			return errors.New("path is invalid")
		}
		return nil
	}
)

func AskInt(msg string, mask bool, validate func(string) error) (int, error) {
	StopSpinner("", logsymbols.Success)
	prompt := promptui.Prompt{
		Label:    msg,
		Validate: validate,
	}
	if mask {
		prompt.Mask = '*'
	}
	output, err := prompt.Run()
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(output)
}

func AskString(msg string, mask bool, validate func(string) error) (string, error) {
	StopSpinner("", logsymbols.Success)
	prompt := promptui.Prompt{
		Label:    msg,
		Validate: validate,
	}
	if mask {
		prompt.Mask = '*'
	}
	return prompt.Run()
}

func AskIP(msg string) (net.IP, error) {
	StopSpinner("", logsymbols.Success)
	prompt := promptui.Prompt{
		Label:    msg,
		Validate: IpValidator,
	}
	ipStr, err := prompt.Run()
	return net.ParseIP(ipStr), err
}

func AskChoice(msg string, choices []string) (string, error) {
	StopSpinner("", logsymbols.Success)
	if len(choices) == 0 {
		return "", errors.New(fmt.Sprintf("Choices is empty for msg: \"%s\"", msg))
	}
	prompt := promptui.Select{
		Label: msg,
		Items: choices,
	}
	_, returnStr, err := prompt.Run()
	return returnStr, err
}

func UserConfirmation(msg string) (bool, error) {
	StopSpinner("", logsymbols.Success)
	prompt := promptui.Prompt{
		Label:    fmt.Sprintf("%s [Y/n]", msg),
		Validate: YesNoValidator,
	}
	result, err := prompt.Run()
	if err != nil {
		return false, err
	}
	if strings.ToLower(result) != "y" && strings.ToLower(result) != "yes" {
		return false, nil
	}
	return true, nil
}

func PrintWarning(msg string) {
	fmt.Printf("%s %s\n", logsymbols.Warning, msg)
}

func GetOrdinalNumber(n int) string {
	switch n % 10 {
	case 1:
		return fmt.Sprintf("%dst", n)
	case 2:
		return fmt.Sprintf("%dnd", n)
	case 3:
		return fmt.Sprintf("%drd", n)
	default:
		return fmt.Sprintf("%dth", n)
	}
}

func GetMajorVersion(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return version
}

type TemplateVars map[string]interface{}

func RenderTemplate(tmpl *template.Template, variables map[string]interface{}) (string, error) {
	var buf strings.Builder
	if err := tmpl.Execute(&buf, variables); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func GetLastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return ""
}
