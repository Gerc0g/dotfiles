package onboard

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"github.com/Gerc0g/dotfiles/core/world"
)

var (
	hostLabel  = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?$`)
	emailLocal = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._%+-]*$`)
)

func validateHost(host string) error {
	if len(host) == 0 || len(host) > 253 {
		return fmt.Errorf("invalid VCS host %q: expected a hostname without scheme, port or path", host)
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) > 63 || !hostLabel.MatchString(label) {
			return fmt.Errorf("invalid VCS host %q: expected a hostname without scheme, port or path", host)
		}
	}
	return nil
}

func validateNamespace(ns string) error {
	for _, part := range strings.Split(ns, "/") {
		if err := world.ValidateSegment(part); err != nil {
			return fmt.Errorf("invalid namespace %q: %w", ns, err)
		}
	}
	return nil
}

func validateEmail(email string) error {
	if email == "" {
		return nil
	}
	address, err := mail.ParseAddress(email)
	local, host, found := strings.Cut(email, "@")
	if err != nil || address.Address != email || address.Name != "" || !found || !emailLocal.MatchString(local) || validateHost(host) != nil {
		return fmt.Errorf("invalid git email %q: expected a plain email address without display name or shell characters", email)
	}
	return nil
}

func validateProductOptions(opts ProductOptions) error {
	for _, field := range []struct{ name, value string }{{"company", opts.Company}, {"product", opts.Product}} {
		if err := world.ValidateSegment(field.value); err != nil {
			return fmt.Errorf("%s: %w", field.name, err)
		}
	}
	for _, repo := range opts.Repos {
		if err := world.ValidateSegment(repo); err != nil {
			return fmt.Errorf("repository: %w", err)
		}
	}
	if opts.Namespace != "" {
		return validateNamespace(opts.Namespace)
	}
	return nil
}

func validateCompanyConfig(config world.Config) error {
	if _, _, err := vcsHost(config.Get("vcs") + ":" + config.Get("host")); err != nil {
		return err
	}
	if sshHost := config.Get("ssh_host"); sshHost != "" {
		// SSH aliases are tokens, not DNS names; generated company slugs may
		// contain underscores. Still reject separators and option injection.
		if err := world.ValidateSegment(sshHost); err != nil {
			return err
		}
		if config.Get("vcs") == "local" && sshHost != "local" {
			return fmt.Errorf("local VCS config cannot select a remote SSH alias")
		}
	}
	if err := validateNamespace(config.Get("namespace")); err != nil {
		return err
	}
	return validateEmail(config.Get("git_email"))
}
