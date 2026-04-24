package tools

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

var emailPattern = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func ValidateEmail(email string) error {
	if !emailPattern.MatchString(email) || utf8.RuneCountInString(email) > 254 {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if utf8.RuneCountInString(password) > 128 {
		return fmt.Errorf("password must be at most 128 characters")
	}
	return nil
}

// ValidateExternalURL checks that a URL is a valid HTTPS URL pointing to a
// public (non-internal) host. Use this for SSO endpoints and any other
// user-configured outbound URLs to prevent SSRF.
func ValidateExternalURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL is required")
	}

	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if !strings.EqualFold(parsed.Scheme, "https") {
		return fmt.Errorf("URL must use HTTPS scheme, got %q", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("URL must include a hostname")
	}

	if host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return fmt.Errorf("URL must not point to internal hosts")
	}

	ips, err := net.LookupHost(host)
	if err != nil {
		return nil
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("URL resolves to a private/internal IP address (%s)", ipStr)
		}
	}

	return nil
}
