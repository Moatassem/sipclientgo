package global

import (
	"fmt"
	"regexp"
	"sipclientgo/system"
	"strings"
)

func GetURIUsername(uri string) string {
	var mtch []string
	if RMatch(uri, NumberOnly, &mtch) {
		return mtch[1]
	}
	return ""
}

func (m Method) IsDialogueCreating() bool {
	switch m {
	case OPTIONS, INVITE: // MESSAGE, NEGOTIATE
		return true
	}
	return false
}

func (m Method) RequiresACK() bool {
	switch m {
	case INVITE, ReINVITE:
		return true
	}
	return false
}

// =====================================================

func (he HeaderEnum) LowerCaseString() string {
	h := HeaderEnumToString[he]
	return system.ASCIIToLower(h)
}

func (he HeaderEnum) String() string {
	return HeaderEnumToString[he]
}

// case insensitive equality with string header name
func (he HeaderEnum) Equals(h string) bool {
	return he.LowerCaseString() == system.ASCIIToLower(h)
}

// =====================================================

func RMatch(s string, rgxfp FieldPattern, mtch *[]string) bool {
	if s == "" {
		return false
	}
	*mtch = DicFieldRegEx[rgxfp].FindStringSubmatch(s)
	return *mtch != nil
}

func RReplace1(input string, rgxfp FieldPattern, replacement string) string {
	return DicFieldRegEx[rgxfp].ReplaceAllString(input, replacement)
}

func RReplaceNumberOnly(input string, replacement string) string {
	return DicFieldRegEx[ReplaceNumberOnly].ReplaceAllString(input, replacement)
}

func TranslateInternal(input string, matches []string) (string, error) {
	if input == "" {
		return "", nil
	}
	if matches == nil {
		return "", fmt.Errorf("empty matches slice")
	}
	sbToInt := func(sb strings.Builder) int {
		return system.Str2Int[int](sb.String())
	}

	item := func(idx int, dblbrkt bool) string {
		if idx >= len(matches) {
			if dblbrkt {
				return fmt.Sprintf("${%v}", idx)
			}
			return fmt.Sprintf("$%v", idx)
		}
		return matches[idx]
	}

	var b strings.Builder
outerloop:
	for i := 0; i < len(input); i++ {
		c := input[i]
		if c == '$' {
			i++
			if i == len(input) {
				b.WriteByte(c)
				return b.String(), nil
			}
			c = input[i]
			if c == '$' {
				b.WriteByte(c)
				continue outerloop
			}
			var grpsb strings.Builder
			for {
				if '0' <= c && c <= '9' {
					grpsb.WriteByte(c)
					i++
					if i == len(input) {
						v := item(sbToInt(grpsb), false)
						b.WriteString(v)
						return b.String(), nil
					}
					c = input[i]
				} else if c == '{' {
					if grpsb.Len() == 0 {
						break
					} else {
						b.WriteByte(c)
						v := item(sbToInt(grpsb), false)
						b.WriteString(v)
						continue outerloop
					}
				} else {
					if grpsb.Len() == 0 {
						b.WriteByte('$')
						b.WriteByte(c)
					} else {
						v := item(sbToInt(grpsb), false)
						b.WriteString(v)
					}
					continue outerloop
				}
			}
			for {
				i++
				if i == len(input) {
					return "", fmt.Errorf("bracket unclosed")
				}
				c = input[i]
				if '0' <= c && c <= '9' {
					grpsb.WriteByte(c)
				} else if c == '}' {
					if grpsb.Len() == 0 {
						return "", fmt.Errorf("bracket closed without group index")
					}
					v := item(sbToInt(grpsb), true)
					b.WriteString(v)
					continue outerloop
				} else if c == '{' {
					b.WriteByte(c)
					continue outerloop
				} else {
					return "", fmt.Errorf("invalid character within bracket")
				}
			}
		}
		b.WriteByte(c)
	}
	return b.String(), nil
}

func TranslateExternal(input string, rgxstring string, trans string) string {
	rgx, err := regexp.Compile(rgxstring)
	if err != nil {
		return ""
	}
	var result []byte
	result = rgx.ExpandString(result, trans, input, rgx.FindStringSubmatchIndex(input))
	return string(result)
}

// Use rgx.FindStringSubmatchIndex(input) to get matches
func TranslateResult(rgx *regexp.Regexp, input string, trans string, matches []int) string {
	var result []byte
	result = rgx.ExpandString(result, trans, input, matches)
	return string(result)
}

func TranslateResult2(input string, rgx *regexp.Regexp, trans string) (string, bool) {
	var (
		result    []byte
		resultStr string
	)

	result = rgx.ExpandString(result, trans, input, rgx.FindStringSubmatchIndex(input))
	resultStr = string(result)

	return resultStr, (resultStr != "" && trans != "") || (resultStr == "" && trans == "")
}

func GetBodyType(contentType string) BodyType {
	contentType = system.ASCIIToLower(contentType)
	for k, v := range DicBodyContentType {
		if v == contentType {
			return k
		}
	}
	if strings.Contains(contentType, "xml") {
		return AnyXML
	}
	return Unknown
}

func HeaderCase(h string) string {
	h = system.ASCIIToLower(h)
	for k := range HeaderStringtoEnum {
		if system.ASCIIToLower(k) == h {
			return k
		}
	}
	return system.ASCIIPascal(h)
}
