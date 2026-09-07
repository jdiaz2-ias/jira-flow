package domain

import (
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var issueKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}-[1-9][0-9]{0,18}$`)

func IssueKey(key string) (string, error) {
	key = strings.ToUpper(key)
	if !issueKeyPattern.MatchString(key) {
		return "", &Error{Kind: InvalidInput, Message: "Invalid issue key; use PROJECT-123."}
	}
	return key, nil
}
func BrowseURL(site, key string) (string, error) {
	key, err := IssueKey(key)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(site)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return "", &Error{Kind: InvalidInput, Message: "Invalid Jira site."}
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/browse/" + key
	return u.String(), nil
}

type QueryOptions struct {
	Mode, JQL, Project, Category, Type, Priority, UpdatedSince, Sort string
	IncludeDone                                                      bool
}

func quoteJQL(s string) (string, error) {
	if len(s) > 256 || strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) }) {
		return "", &Error{Kind: InvalidInput, Message: "Invalid filter value."}
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`, nil
}
func (o QueryOptions) Build() (string, error) {
	bad := func(s string) (string, error) { return "", &Error{Kind: InvalidInput, Message: s} }
	if o.Mode == "search" {
		if o.Project != "" || o.Category != "" || o.Type != "" || o.Priority != "" || o.UpdatedSince != "" || o.Sort != "" || o.IncludeDone {
			return bad("Do not combine --jql with structured filters or --sort.")
		}
		if strings.TrimSpace(o.JQL) == "" || len(o.JQL) > 32768 {
			return bad("Provide a non-empty --jql query up to 32 KiB.")
		}
		return o.JQL, nil
	}
	if o.Mode != "mine" && o.Mode != "list" {
		return bad("Invalid query type.")
	}
	if o.Mode == "list" && o.Project == "" && o.Category == "" && o.Type == "" && o.Priority == "" && o.UpdatedSince == "" {
		return bad("list requires --project, a default project, or another explicit filter.")
	}
	parts := []string{}
	if o.Mode == "mine" {
		parts = append(parts, "assignee = currentUser()")
	}
	for _, f := range []struct{ key, value string }{{"project", o.Project}, {"issuetype", o.Type}, {"priority", o.Priority}} {
		if f.value != "" {
			value, e := quoteJQL(f.value)
			if e != nil {
				return "", e
			}
			parts = append(parts, f.key+" = "+value)
		}
	}
	if o.Category != "" {
		literal, ok := map[string]string{"todo": "To Do", "in-progress": "In Progress", "done": "Done"}[o.Category]
		if !ok {
			return bad("Invalid category: use todo, in-progress, or done.")
		}
		value, _ := quoteJQL(literal)
		parts = append(parts, "statusCategory = "+value)
	} else if !o.IncludeDone && o.Mode == "mine" {
		parts = append(parts, "statusCategory != Done")
	}
	if o.UpdatedSince != "" {
		v := o.UpdatedSince
		if _, err := time.Parse("2006-01-02", v); err != nil {
			if !regexp.MustCompile(`^-[1-9][0-9]{0,3}[mhdw]$`).MatchString(v) {
				return bad("--updated-since requires YYYY-MM-DD or an interval like -7d.")
			}
		}
		value, _ := quoteJQL(v)
		parts = append(parts, "updated >= "+value)
	}
	order := []string{"updated DESC", "key ASC"}
	if o.Sort != "" {
		order = nil
		seen := map[string]bool{}
		for _, item := range strings.Split(o.Sort, ",") {
			direction := "ASC"
			if strings.HasPrefix(item, "-") {
				direction = "DESC"
				item = strings.TrimPrefix(item, "-")
			}
			field, ok := map[string]string{"updated": "updated", "created": "created", "priority": "priority", "key": "key", "due": "due"}[item]
			if !ok || seen[field] {
				return bad("--sort accepts updated, created, priority, key, or due; use - for descending.")
			}
			seen[field] = true
			order = append(order, field+" "+direction)
		}
		if !seen["key"] {
			order = append(order, "key ASC")
		}
	}
	return strings.Join(parts, " AND ") + " ORDER BY " + strings.Join(order, ", "), nil
}

// CleanText removes terminal control sequences, including OSC hyperlinks.
func CleanText(s string, multiline bool) string {
	var b strings.Builder
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		c := r[i]
		if c == 0x1b || c == 0x9b || c == 0x9d {
			kind := c
			if c == 0x1b {
				if i+1 >= len(r) {
					break
				}
				i++
				kind = r[i]
			}
			if kind == '[' || kind == 0x9b {
				for i+1 < len(r) {
					i++
					if r[i] >= 0x40 && r[i] <= 0x7e {
						break
					}
				}
			} else if kind == ']' || kind == 0x9d || kind == 'P' || kind == '_' || kind == '^' {
				for i+1 < len(r) {
					i++
					if r[i] == 7 || r[i] == 0x9c {
						break
					}
					if r[i] == 0x1b && i+1 < len(r) && r[i+1] == '\\' {
						i++
						break
					}
				}
			}
			continue
		}
		if c == '\n' || c == '\t' {
			if multiline {
				b.WriteRune(c)
			} else {
				b.WriteByte(' ')
			}
			continue
		}
		if unicode.IsControl(c) || unicode.Is(unicode.Cf, c) {
			continue
		}
		b.WriteRune(c)
	}
	return b.String()
}
