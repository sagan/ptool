package impersonateutil

import (
	"fmt"

	"github.com/sagan/ptool/util"
)

type Profile struct {
	// "chrome", "firefox", "opera", "safari", "edge", "ios", "android"
	Name          string
	Navigator     string
	Ja3           string
	H2fingerpring string
	Headers       [][]string // use "\n" as placeholder for order; use "" (empty) to delete a header
	Comment       string
}

var (
	// all supported impersonate names
	profilesMap = map[string]*Profile{}
)

// GetProfile get impersonate profile by name. If name is empty, return default profile.
// If name is not empty but corresponding profile does not exists, return nil.
func GetProfile(name string) *Profile {
	if name == "" {
		return profilesMap[DEFAULT_IMPERSONATE]
	}
	return profilesMap[name]
}

// Get all available impersonate profile names, joined by ","
func GetAllProfileNames() (names string) {
	sep := ""
	for _, p := range profiles {
		names += fmt.Sprintf("%s%s", sep, p.Name)
		sep = ", "
	}
	return
}

func init() {
	for _, impersonateProfile := range profiles {
		profilesMap[impersonateProfile.Name] = impersonateProfile
	}
}

func (p *Profile) Print() {
	fmt.Printf("Impersonate '%s'\n", p.Name)
	fmt.Printf("- navigator: %s\n", p.Navigator)
	fmt.Printf("- comment: %s\n", p.Comment)
	fmt.Printf("- tls_ja3: %s\n", p.Ja3)
	fmt.Printf("- h2_fingerprint: %s\n", p.H2fingerpring)
	fmt.Printf("- http_request_headers:\n")
	for _, header := range p.Headers {
		value := header[1]
		if value == util.HTTP_HEADER_PLACEHOLDER {
			value = ""
		}
		fmt.Printf("  %s: %s\n", header[0], value)
	}
}
