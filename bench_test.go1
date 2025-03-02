package main

import (
	"regexp"
	"strings"
	"testing"
)

type AuthScheme struct {
	Scheme string
	Params map[string]string
}

// Initial algorithm
func ParseWWWAuthenticateInitial(header string) []AuthScheme {
	var schemes []AuthScheme
	schemeRegex := regexp.MustCompile(`(?i)^(Basic|Bearer|Digest|NTLM|OAuth|Negotiate)\b`)
	var currentScheme *AuthScheme

	inQuotes := false
	var partBuilder strings.Builder
	var parts []string

	for _, char := range header {
		switch char {
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				parts = append(parts, strings.TrimSpace(partBuilder.String()))
				partBuilder.Reset()
				continue
			}
		}
		partBuilder.WriteRune(char)
	}
	if partBuilder.Len() > 0 {
		parts = append(parts, strings.TrimSpace(partBuilder.String()))
	}

	for _, part := range parts {
		if match := schemeRegex.FindString(part); match != "" {
			if currentScheme != nil {
				schemes = append(schemes, *currentScheme)
			}
			currentScheme = &AuthScheme{Scheme: match, Params: make(map[string]string)}
			part = strings.TrimSpace(strings.TrimPrefix(part, match))
		}

		if currentScheme == nil {
			continue
		}

		paramRegex := regexp.MustCompile(`([a-zA-Z0-9_-]+)\s*=\s*(?:"([^"]+)"|([^,]+))`)
		paramMatches := paramRegex.FindAllStringSubmatch(part, -1)

		for _, pm := range paramMatches {
			key := strings.TrimSpace(pm[1])
			var value string
			if pm[2] != "" {
				value = pm[2]
			} else {
				value = strings.TrimSpace(pm[3])
			}
			currentScheme.Params[key] = value
		}
	}

	if currentScheme != nil {
		schemes = append(schemes, *currentScheme)
	}

	return schemes
}

// Optimized algorithm
func ParseWWWAuthenticateOptimized(header string) []AuthScheme {
	var schemes []AuthScheme
	schemeRegex := regexp.MustCompile(`(?i)^(Basic|Bearer|Digest|NTLM|OAuth|Negotiate)\b`)
	paramRegex := regexp.MustCompile(`([a-zA-Z0-9_-]+)\s*=\s*("[^"]+"|[^,]+)`)

	parts := splitHeader(header)

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if match := schemeRegex.FindString(part); match != "" {
			schemes = append(schemes, AuthScheme{
				Scheme: match,
				Params: extractParams(strings.TrimSpace(strings.TrimPrefix(part, match)), paramRegex),
			})
		} else if len(schemes) > 0 {
			currentScheme := &schemes[len(schemes)-1]
			for key, value := range extractParams(part, paramRegex) {
				currentScheme.Params[key] = value
			}
		}
	}

	return schemes
}

func splitHeader(header string) []string {
	var parts []string
	inQuotes := false
	var partBuilder strings.Builder

	for _, char := range header {
		switch char {
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				parts = append(parts, partBuilder.String())
				partBuilder.Reset()
				continue
			}
		}
		partBuilder.WriteRune(char)
	}
	if partBuilder.Len() > 0 {
		parts = append(parts, partBuilder.String())
	}

	return parts
}

func extractParams(part string, paramRegex *regexp.Regexp) map[string]string {
	params := make(map[string]string)
	for _, pm := range paramRegex.FindAllStringSubmatch(part, -1) {
		key := strings.TrimSpace(pm[1])
		value := pm[2]
		params[key] = strings.Trim(value, `"`)
	}
	return params
}

func BenchmarkParseWWWAuthenticateInitial(b *testing.B) {
	header := `Basic realm="example", Bearer token="abc,123", Digest qop="auth", charset="utf-8", NTLM, OAuth token="xyz,456", Negotiate`
	for i := 0; i < b.N; i++ {
		ParseWWWAuthenticateInitial(header)
	}
}

func BenchmarkParseWWWAuthenticateOptimized(b *testing.B) {
	header := `Basic realm="example", Bearer token="abc,123", Digest qop="auth", charset="utf-8", NTLM, OAuth token="xyz,456", Negotiate`
	for i := 0; i < b.N; i++ {
		ParseWWWAuthenticateOptimized(header)
	}
}

// func main() {
// 	header := `Basic realm="example", Bearer token="abc,123", Digest qop="auth", charset="utf-8", NTLM, OAuth token="xyz,456", Negotiate`

// 	fmt.Println("Initial Algorithm Result:")
// 	authSchemesInitial := ParseWWWAuthenticateInitial(header)
// 	for _, scheme := range authSchemesInitial {
// 		fmt.Printf("Scheme: %s\n", scheme.Scheme)
// 		for key, value := range scheme.Params {
// 			fmt.Printf("  %s: %s\n", key, value)
// 		}
// 	}

// 	fmt.Println("\nOptimized Algorithm Result:")
// 	authSchemesOptimized := ParseWWWAuthenticateOptimized(header)
// 	for _, scheme := range authSchemesOptimized {
// 		fmt.Printf("Scheme: %s\n", scheme.Scheme)
// 		for key, value := range scheme.Params {
// 			fmt.Printf("  %s: %s\n", key, value)
// 		}
// 	}
// }
