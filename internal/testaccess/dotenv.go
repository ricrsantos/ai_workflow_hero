// Package testaccess parses and validates project-local browser test accounts.
// It treats dotenv files as data and never evaluates shell expressions.
package testaccess

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

const (
	usersKey   = "HERO_TEST_USERS"
	userPrefix = "HERO_TEST_USER_"
)

var errInvalidUserID = errors.New("user ID must match [a-z][a-z0-9_]*")

// ParseError describes a dotenv syntax or schema problem without retaining or
// exposing the assignment value.
type ParseError struct {
	Line   int
	Field  string
	Reason string
}

func (e *ParseError) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("line %d: %s", e.Line, e.Reason)
	}
	return fmt.Sprintf("line %d: %s: %s", e.Line, e.Field, e.Reason)
}

// Account is a parsed test account. Its credential fields are private so
// ordinary formatting and JSON encoding cannot accidentally reveal them.
type Account struct {
	id       string
	login    string
	password string
	profile  string
}

// NewAccount creates an account draft. Empty fields are allowed so a user
// editor can represent an incomplete draft; ValidateRequiredAccount checks
// usability before execution.
func NewAccount(id, login, password, profile string) (Account, error) {
	if _, err := ValidateUserID(id); err != nil {
		return Account{}, err
	}
	return Account{id: id, login: login, password: password, profile: profile}, nil
}

// ID returns the stable, lowercase account ID.
func (a Account) ID() string { return a.id }

// Login returns the account login value.
func (a Account) Login() string { return a.login }

// Password returns the account password value to trusted test-access callers.
func (a Account) Password() string { return a.password }

// Profile returns the account role/profile value.
func (a Account) Profile() string { return a.profile }

// String implements fmt.Stringer with a redacted representation.
func (Account) String() string { return "testaccess.Account[redacted]" }

// GoString keeps %#v formatting redacted as well.
func (Account) GoString() string { return "testaccess.Account[redacted]" }

// Format prevents fmt verbs from formatting credential fields.
func (a Account) Format(state fmt.State, verb rune) {
	formatRedacted(state, verb, "testaccess.Account[redacted]")
}

// MarshalJSON keeps account credentials out of JSON representations.
func (Account) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

// Document is a parsed dotenv file. Unrelated assignments, comments, and line
// endings are retained when account entries are updated.
type Document struct {
	lines        []dotenvLine
	accounts     []Account
	lineEnding   string
	finalNewline bool
}

type dotenvLine struct {
	content string
	ending  string
	managed bool
}

type assignment struct {
	key     string
	value   string
	line    int
	managed bool
}

// ParseDotenv parses supported dotenv syntax without expanding variables or
// executing shell content. Managed test-user keys are validated as a schema.
func ParseDotenv(data []byte) (Document, error) {
	lines, lineEnding, finalNewline := splitLines(data)
	assignments := make(map[string]assignment)
	seen := make(map[string]struct{})
	managedLines := make(map[int]bool)

	for i, line := range lines {
		key, value, isAssignment, err := parseLine(line.content, i+1)
		if err != nil {
			return Document{}, err
		}
		if !isAssignment {
			continue
		}

		managed := isManagedKey(key)
		identity := key
		if managed {
			identity = strings.ToUpper(key)
		}
		if _, exists := seen[identity]; exists {
			reason := "duplicate key"
			if managed {
				reason = "duplicate or ambiguously normalized key"
			}
			return Document{}, parseError(i+1, safeField(identity), reason)
		}
		seen[identity] = struct{}{}
		managedLines[i] = managed
		assignments[identity] = assignment{key: key, value: value, line: i + 1, managed: managed}

		if managed && key != identity {
			return Document{}, parseError(i+1, safeField(identity), "ambiguous or non-canonical managed field normalization")
		}
	}

	accounts, err := parseAccounts(assignments)
	if err != nil {
		return Document{}, err
	}
	for i := range lines {
		lines[i].managed = managedLines[i]
	}
	return Document{
		lines:        lines,
		accounts:     accounts,
		lineEnding:   lineEnding,
		finalNewline: finalNewline,
	}, nil
}

// SerializeDotenv serializes a document, retaining source formatting where it
// has not been changed. Account values created through SetAccounts are
// double-quoted with only the documented escapes.
func SerializeDotenv(document Document) ([]byte, error) {
	var output bytes.Buffer
	for _, line := range document.lines {
		output.WriteString(line.content)
		output.WriteString(line.ending)
	}
	serialized := output.Bytes()
	if _, err := ParseDotenv(serialized); err != nil {
		return nil, fmt.Errorf("serializing dotenv: %w", err)
	}
	return serialized, nil
}

// Accounts returns configured accounts in HERO_TEST_USERS order. The returned
// slice is a copy; each Account redacts itself when formatted.
func (d Document) Accounts() []Account {
	return append([]Account(nil), d.accounts...)
}

// SelectedAccount returns a usable account by exact stable ID. Missing or
// incomplete accounts produce value-free errors.
func (d Document) SelectedAccount(id string) (Account, error) {
	if _, err := ValidateUserID(id); err != nil {
		return Account{}, err
	}
	for _, account := range d.accounts {
		if account.id != id {
			continue
		}
		if err := ValidateRequiredAccount(account); err != nil {
			return Account{}, err
		}
		return account, nil
	}
	return Account{}, errors.New("selected test account is not configured")
}

// SetAccounts replaces the managed account block while preserving unrelated
// entries and standalone comments. Drafts may be incomplete.
func (d *Document) SetAccounts(accounts []Account) error {
	if d == nil {
		return errors.New("cannot update a nil dotenv document")
	}
	ids := make(map[string]struct{}, len(accounts))
	for _, account := range accounts {
		if _, err := ValidateUserID(account.id); err != nil {
			return err
		}
		for _, value := range []struct {
			name  string
			value string
		}{
			{name: "login", value: account.login},
			{name: "password", value: account.password},
			{name: "profile", value: account.profile},
		} {
			if hasUnsupportedControl(value.value) {
				return fmt.Errorf("test account %s contains an unsupported control character", value.name)
			}
		}
		if _, exists := ids[account.id]; exists {
			return errors.New("duplicate test user ID")
		}
		ids[account.id] = struct{}{}
	}

	filtered := make([]dotenvLine, 0, len(d.lines))
	insertion := -1
	for _, line := range d.lines {
		if line.managed {
			if insertion < 0 {
				insertion = len(filtered)
			}
			continue
		}
		filtered = append(filtered, line)
	}
	if insertion < 0 {
		insertion = len(filtered)
	}

	generated := make([]dotenvLine, 0, 1+3*len(accounts))
	if len(accounts) > 0 {
		userIDs := make([]string, 0, len(accounts))
		for _, account := range accounts {
			userIDs = append(userIDs, account.id)
		}
		generated = append(generated, dotenvLine{
			content: usersKey + "=" + strings.Join(userIDs, ","),
			managed: true,
		})
		for _, account := range accounts {
			suffix, _ := ValidateUserID(account.id)
			generated = append(generated,
				dotenvLine{content: userPrefix + suffix + "_LOGIN=" + quoteValue(account.login), managed: true},
				dotenvLine{content: userPrefix + suffix + "_PASSWORD=" + quoteValue(account.password), managed: true},
				dotenvLine{content: userPrefix + suffix + "_PROFILE=" + quoteValue(account.profile), managed: true},
			)
		}
	}

	updated := make([]dotenvLine, 0, len(filtered)+len(generated))
	updated = append(updated, filtered[:insertion]...)
	updated = append(updated, generated...)
	updated = append(updated, filtered[insertion:]...)
	if d.lineEnding == "" {
		d.lineEnding = "\n"
	}
	for i := range updated {
		if i < len(updated)-1 && updated[i].ending == "" {
			updated[i].ending = d.lineEnding
		}
	}
	if len(updated) > 0 {
		if d.finalNewline {
			updated[len(updated)-1].ending = d.lineEnding
		} else {
			updated[len(updated)-1].ending = ""
		}
	}
	d.lines = updated
	d.accounts = append([]Account(nil), accounts...)
	return nil
}

// ValidateUserID checks the stable account ID format and returns the uppercase
// suffix used in dotenv keys.
func ValidateUserID(id string) (string, error) {
	if id == "" || id[0] < 'a' || id[0] > 'z' {
		return "", errInvalidUserID
	}
	for i := 1; i < len(id); i++ {
		c := id[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return "", errInvalidUserID
		}
	}
	return strings.ToUpper(id), nil
}

// ValidateRequiredAccount confirms an account has non-empty login, password,
// and profile fields before it is used for an authenticated test.
func ValidateRequiredAccount(account Account) error {
	if _, err := ValidateUserID(account.id); err != nil {
		return err
	}
	if account.login == "" {
		return errors.New("test account login is required")
	}
	if account.password == "" {
		return errors.New("test account password is required")
	}
	if account.profile == "" {
		return errors.New("test account profile is required")
	}
	return nil
}

// String implements fmt.Stringer without exposing source lines or accounts.
func (Document) String() string { return "testaccess.Document[redacted]" }

// GoString keeps %#v formatting redacted as well.
func (Document) GoString() string { return "testaccess.Document[redacted]" }

// Format prevents fmt verbs from formatting dotenv assignments.
func (Document) Format(state fmt.State, verb rune) {
	formatRedacted(state, verb, "testaccess.Document[redacted]")
}

// MarshalJSON emits only non-sensitive collection counts.
func (d Document) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		AccountCount int `json:"account_count"`
		EntryCount   int `json:"entry_count"`
	}{AccountCount: len(d.accounts), EntryCount: len(d.lines)})
}

func parseAccounts(assignments map[string]assignment) ([]Account, error) {
	users, hasUsers := assignments[usersKey]
	userIDs := make([]string, 0)
	accountIndex := make(map[string]int)
	if hasUsers && strings.TrimSpace(users.value) != "" {
		for _, part := range strings.Split(users.value, ",") {
			id := strings.TrimSpace(part)
			if id == "" {
				return nil, parseError(users.line, usersKey, "empty user ID")
			}
			if _, err := ValidateUserID(id); err != nil {
				return nil, parseError(users.line, usersKey, "invalid user ID")
			}
			if _, exists := accountIndex[id]; exists {
				return nil, parseError(users.line, usersKey, "duplicate user ID")
			}
			accountIndex[id] = len(userIDs)
			userIDs = append(userIDs, id)
		}
	}

	accounts := make([]Account, len(userIDs))
	for i, id := range userIDs {
		accounts[i].id = id
	}
	entries := make([]assignment, 0, len(assignments))
	for _, entry := range assignments {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].line < entries[j].line })
	for _, entry := range entries {
		key := entry.key
		if !entry.managed || key == usersKey {
			continue
		}
		remainder := strings.TrimPrefix(key, userPrefix)
		separator := strings.LastIndexByte(remainder, '_')
		if separator <= 0 {
			return nil, parseError(entry.line, "test-user field", "invalid test-user field name")
		}
		suffix := remainder[:separator]
		field := remainder[separator+1:]
		if field != "LOGIN" && field != "PASSWORD" && field != "PROFILE" {
			return nil, parseError(entry.line, "test-user field", "unknown test-user field")
		}
		id := strings.ToLower(suffix)
		if _, err := ValidateUserID(id); err != nil || strings.ToUpper(id) != suffix {
			return nil, parseError(entry.line, "test-user ID", "ambiguous or invalid user ID normalization")
		}
		if hasUnsupportedControl(entry.value) {
			return nil, parseError(entry.line, safeField(key), "unsupported control character")
		}
		index, exists := accountIndex[id]
		if !exists {
			reason := "test-user fields require HERO_TEST_USERS"
			if hasUsers {
				reason = "test-user ID is not listed in HERO_TEST_USERS"
			}
			return nil, parseError(entry.line, safeField(key), reason)
		}
		switch field {
		case "LOGIN":
			accounts[index].login = entry.value
		case "PASSWORD":
			accounts[index].password = entry.value
		case "PROFILE":
			accounts[index].profile = entry.value
		}
	}
	return accounts, nil
}

func parseLine(line string, lineNumber int) (key, value string, isAssignment bool, err error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false, nil
	}

	start := 0
	for start < len(line) && (line[start] == ' ' || line[start] == '\t') {
		start++
	}
	equals := strings.IndexByte(line[start:], '=')
	if equals < 0 {
		return "", "", false, parseError(lineNumber, "assignment", "expected KEY=VALUE")
	}
	equals += start
	key = strings.TrimSpace(line[start:equals])
	if !validEnvKey(key) {
		return "", "", false, parseError(lineNumber, "field name", "invalid environment key")
	}
	raw := strings.TrimLeft(line[equals+1:], " \t")
	value, err = parseValue(raw, lineNumber, safeField(key))
	if err != nil {
		return "", "", false, err
	}
	return key, value, true, nil
}

func parseValue(raw string, lineNumber int, key string) (string, error) {
	if raw == "" {
		return "", nil
	}
	switch raw[0] {
	case '\'':
		end := strings.IndexByte(raw[1:], '\'')
		if end < 0 {
			return "", parseError(lineNumber, key, "unterminated single-quoted value")
		}
		value := raw[1 : end+1]
		if err := validateQuotedRemainder(raw[end+2:], lineNumber, key); err != nil {
			return "", err
		}
		return value, nil
	case '"':
		return parseDoubleQuoted(raw, lineNumber, key)
	default:
		if comment := strings.IndexByte(raw, '#'); comment >= 0 {
			raw = raw[:comment]
		}
		return strings.TrimSpace(raw), nil
	}
}

func parseDoubleQuoted(raw string, lineNumber int, key string) (string, error) {
	var value strings.Builder
	for i := 1; i < len(raw); i++ {
		c := raw[i]
		switch c {
		case '"':
			if err := validateQuotedRemainder(raw[i+1:], lineNumber, key); err != nil {
				return "", err
			}
			return value.String(), nil
		case '\\':
			i++
			if i >= len(raw) {
				return "", parseError(lineNumber, key, "unterminated escape sequence")
			}
			switch raw[i] {
			case 'n':
				value.WriteByte('\n')
			case 'r':
				value.WriteByte('\r')
			case 't':
				value.WriteByte('\t')
			case '"':
				value.WriteByte('"')
			case '\\':
				value.WriteByte('\\')
			default:
				return "", parseError(lineNumber, key, "unsupported escape sequence")
			}
		default:
			value.WriteByte(c)
		}
	}
	return "", parseError(lineNumber, key, "unterminated double-quoted value")
}

func validateQuotedRemainder(remainder string, lineNumber int, key string) error {
	remainder = strings.TrimLeft(remainder, " \t")
	if remainder == "" || strings.HasPrefix(remainder, "#") {
		return nil
	}
	return parseError(lineNumber, key, "unexpected content after quoted value")
}

func validEnvKey(key string) bool {
	if key == "" || (!isASCIIAlpha(key[0]) && key[0] != '_') {
		return false
	}
	for i := 1; i < len(key); i++ {
		c := key[i]
		if !isASCIIAlpha(c) && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}

func isASCIIAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isManagedKey(key string) bool {
	normalized := strings.ToUpper(key)
	return normalized == usersKey || strings.HasPrefix(normalized, userPrefix)
}

func parseError(line int, field, reason string) error {
	return &ParseError{Line: line, Field: safeField(field), Reason: reason}
}

func safeField(field string) string {
	switch field {
	case "assignment", "field name", "environment field", "test-user field", "test-user ID", "test-user login", "test-user password", "test-user profile", usersKey:
		return field
	}
	normalized := strings.ToUpper(field)
	if normalized == usersKey {
		return usersKey
	}
	if !strings.HasPrefix(normalized, userPrefix) {
		if validEnvKey(field) {
			return "environment field"
		}
		return "field name"
	}
	remainder := strings.TrimPrefix(normalized, userPrefix)
	separator := strings.LastIndexByte(remainder, '_')
	if separator <= 0 {
		return "test-user field"
	}
	switch remainder[separator+1:] {
	case "LOGIN":
		return "test-user login"
	case "PASSWORD":
		return "test-user password"
	case "PROFILE":
		return "test-user profile"
	default:
		return "test-user field"
	}
}

func hasUnsupportedControl(value string) bool {
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c == 0x7f || (c < 0x20 && c != '\n' && c != '\r' && c != '\t') {
			return true
		}
	}
	return false
}

func quoteValue(value string) string {
	var quoted strings.Builder
	quoted.Grow(len(value) + 2)
	quoted.WriteByte('"')
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '\n':
			quoted.WriteString(`\n`)
		case '\r':
			quoted.WriteString(`\r`)
		case '\t':
			quoted.WriteString(`\t`)
		case '"':
			quoted.WriteString(`\"`)
		case '\\':
			quoted.WriteString(`\\`)
		default:
			quoted.WriteByte(value[i])
		}
	}
	quoted.WriteByte('"')
	return quoted.String()
}

func splitLines(data []byte) ([]dotenvLine, string, bool) {
	if len(data) == 0 {
		return nil, "\n", false
	}
	lines := make([]dotenvLine, 0, bytes.Count(data, []byte{'\n'})+1)
	lineEnding := "\n"
	finalNewline := data[len(data)-1] == '\n'
	for start := 0; start < len(data); {
		newline := bytes.IndexByte(data[start:], '\n')
		if newline < 0 {
			lines = append(lines, dotenvLine{content: string(data[start:])})
			break
		}
		end := start + newline
		content := data[start:end]
		ending := "\n"
		if len(content) > 0 && content[len(content)-1] == '\r' {
			content = content[:len(content)-1]
			ending = "\r\n"
		}
		if len(lines) == 0 {
			lineEnding = ending
		}
		lines = append(lines, dotenvLine{content: string(content), ending: ending})
		start = end + 1
	}
	return lines, lineEnding, finalNewline
}

func formatRedacted(state fmt.State, verb rune, representation string) {
	if verb == 'q' {
		_, _ = fmt.Fprintf(state, "%q", representation)
		return
	}
	_, _ = io.WriteString(state, representation)
}
