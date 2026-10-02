package testaccess

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestDotenvParsesSupportedValues(t *testing.T) {
	input := []byte("\n# test accounts\nPLAIN=plain-value\nTRAILING=bare # comment\nHERO_TEST_USERS=operator\n" +
		"HERO_TEST_USER_OPERATOR_LOGIN=' login # $HOME '\n" +
		"HERO_TEST_USER_OPERATOR_PASSWORD=\"line\\nreturn\\rtab\\tquote\\\"slash\\\\hash#$HOME\" # ignored\n" +
		"HERO_TEST_USER_OPERATOR_PROFILE=operator\n")

	document, err := ParseDotenv(input)
	if err != nil {
		t.Fatalf("ParseDotenv() error = %v", err)
	}
	if got, want := len(document.Accounts()), 1; got != want {
		t.Fatalf("Accounts() length = %d, want %d", got, want)
	}
	account, err := document.SelectedAccount("operator")
	if err != nil {
		t.Fatalf("SelectedAccount() error = %v", err)
	}
	if got, want := account.Login(), " login # $HOME "; got != want {
		t.Error("Login() did not preserve the quoted value")
	}
	if got, want := account.Password(), "line\nreturn\rtab\tquote\"slash\\hash#$HOME"; got != want {
		t.Error("Password() did not decode the supported escapes")
	}
	if got, want := account.Profile(), "operator"; got != want {
		t.Error("Profile() did not preserve the value")
	}
	if _, err := document.SelectedAccount("missing"); err == nil {
		t.Fatal("SelectedAccount(missing) succeeded")
	}
}

func TestDotenvRejectsMalformedValuesWithoutLeaking(t *testing.T) {
	const secret = "sentinel-dotenv-secret-91c2"
	tests := []struct {
		name      string
		input     string
		wantField string
	}{
		{name: "unsupported escape", input: "HERO_TEST_USER_OPERATOR_PASSWORD=\"" + secret + "\\q\"", wantField: "test-user password"},
		{name: "unterminated double quote", input: "HERO_TEST_USER_OPERATOR_PASSWORD=\"" + secret, wantField: "test-user password"},
		{name: "unterminated single quote", input: "HERO_TEST_USER_OPERATOR_PASSWORD='" + secret, wantField: "test-user password"},
		{name: "unexpected quoted suffix", input: "HERO_TEST_USER_OPERATOR_PASSWORD=\"" + secret + "\"suffix", wantField: "test-user password"},
		{name: "malicious field name", input: "HERO_TEST_USER_PASSWORD!=" + secret, wantField: "field name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDotenv([]byte(tt.input))
			if err == nil {
				t.Fatal("ParseDotenv() succeeded for malformed input")
			}
			message := err.Error()
			if !strings.Contains(message, "line 1") || !strings.Contains(message, tt.wantField) {
				t.Errorf("error %q does not identify line and field %q", message, tt.wantField)
			}
			if strings.Contains(message, secret) || strings.Contains(message, "HERO_TEST_USER_PASSWORD!") {
				t.Errorf("diagnostic leaked a value or invalid field: %q", message)
			}
		})
	}
}

func TestDotenvRejectsDuplicateAndAmbiguousEntries(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "duplicate key", input: "OTHER=value\nOTHER=again"},
		{name: "duplicate account ID", input: "HERO_TEST_USERS=operator,operator"},
		{name: "case-normalized managed key", input: "HERO_TEST_USERS=operator\nHERO_TEST_USER_OPERATOR_LOGIN=a\nHERO_TEST_USER_operator_PASSWORD=b"},
		{name: "unlisted account fields", input: "HERO_TEST_USERS=operator\nHERO_TEST_USER_ADMIN_LOGIN=hidden"},
		{name: "invalid user suffix", input: "HERO_TEST_USERS=operator\nHERO_TEST_USER_2OPERATOR_LOGIN=hidden"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDotenv([]byte(tt.input))
			if err == nil {
				t.Fatal("ParseDotenv() succeeded for invalid input")
			}
			if strings.Contains(err.Error(), "hidden") || strings.Contains(err.Error(), "again") {
				t.Errorf("diagnostic leaked assignment value: %q", err)
			}
		})
	}
}

func TestDotenvPreservesUnchangedSource(t *testing.T) {
	input := []byte("# first\r\nUNRELATED = 'literal # value'\r\n\r\n# last")
	document, err := ParseDotenv(input)
	if err != nil {
		t.Fatalf("ParseDotenv() error = %v", err)
	}
	got, err := SerializeDotenv(document)
	if err != nil {
		t.Fatalf("SerializeDotenv() error = %v", err)
	}
	if string(got) != string(input) {
		t.Errorf("SerializeDotenv() changed unchanged source:\n got %q\nwant %q", got, input)
	}
}

func TestDotenvIncompleteAccountIsAValidDraft(t *testing.T) {
	document, err := ParseDotenv([]byte("HERO_TEST_USERS=operator\nHERO_TEST_USER_OPERATOR_LOGIN=login"))
	if err != nil {
		t.Fatalf("ParseDotenv() error = %v", err)
	}
	if _, err := document.SelectedAccount("operator"); err == nil || !strings.Contains(err.Error(), "password") {
		t.Fatalf("SelectedAccount() error = %v, want missing password", err)
	}
}

func TestUserIDValidationAndSuffix(t *testing.T) {
	valid := []struct {
		id     string
		suffix string
	}{
		{id: "a", suffix: "A"},
		{id: "operator", suffix: "OPERATOR"},
		{id: "admin_user2", suffix: "ADMIN_USER2"},
	}
	for _, tt := range valid {
		t.Run(tt.id, func(t *testing.T) {
			got, err := ValidateUserID(tt.id)
			if err != nil {
				t.Fatalf("ValidateUserID() error = %v", err)
			}
			if got != tt.suffix {
				t.Errorf("ValidateUserID() suffix = %q, want %q", got, tt.suffix)
			}
		})
	}
	invalid := []string{"", "Operator", "2operator", "operator-admin", "operator.admin", "a b", "éclair"}
	for _, id := range invalid {
		t.Run(fmt.Sprintf("invalid_%q", id), func(t *testing.T) {
			if _, err := ValidateUserID(id); err == nil {
				t.Fatalf("ValidateUserID(%q) succeeded", id)
			}
		})
	}
}

func TestUserIDRequiredAccountUsability(t *testing.T) {
	tests := []struct {
		name     string
		login    string
		password string
		profile  string
		missing  string
	}{
		{name: "missing login", password: "password", profile: "operator", missing: "login"},
		{name: "missing password", login: "login", profile: "operator", missing: "password"},
		{name: "missing profile", login: "login", password: "password", missing: "profile"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account, err := NewAccount("operator", tt.login, tt.password, tt.profile)
			if err != nil {
				t.Fatalf("NewAccount() error = %v", err)
			}
			if err := ValidateRequiredAccount(account); err == nil || !strings.Contains(err.Error(), tt.missing) {
				t.Fatalf("ValidateRequiredAccount() error = %v, want missing %s", err, tt.missing)
			}
		})
	}
	account, err := NewAccount("operator", "login", "password", "operator")
	if err != nil {
		t.Fatalf("NewAccount() error = %v", err)
	}
	if err := ValidateRequiredAccount(account); err != nil {
		t.Errorf("ValidateRequiredAccount() error = %v", err)
	}
}

func TestRoundTripComplexValuesWithoutEvaluation(t *testing.T) {
	login := "  user #1 $HOME $(touch nope)  "
	password := "two lines\nand a return\rwith tab\tquote \" and 'single quote' slash \\ dollar $TOKEN #"
	profile := "role with spaces # and $HOME"
	account, err := NewAccount("operator", login, password, profile)
	if err != nil {
		t.Fatalf("NewAccount() error = %v", err)
	}
	document, err := ParseDotenv([]byte("# keep this note\nUNRELATED=value # preserved\n"))
	if err != nil {
		t.Fatalf("ParseDotenv() error = %v", err)
	}
	if err := document.SetAccounts([]Account{account}); err != nil {
		t.Fatalf("SetAccounts() error = %v", err)
	}
	serialized, err := SerializeDotenv(document)
	if err != nil {
		t.Fatalf("SerializeDotenv() error = %v", err)
	}
	if !strings.Contains(string(serialized), `\n`) || !strings.Contains(string(serialized), `$(touch nope)`) {
		t.Errorf("serialized dotenv did not encode newlines and preserve literal shell text: %q", serialized)
	}
	if !strings.Contains(string(serialized), "# keep this note") || !strings.Contains(string(serialized), "UNRELATED=value # preserved") {
		t.Errorf("serialized dotenv lost unrelated source content: %q", serialized)
	}

	reloaded, err := ParseDotenv(serialized)
	if err != nil {
		t.Fatalf("ParseDotenv(round trip) error = %v", err)
	}
	selected, err := reloaded.SelectedAccount("operator")
	if err != nil {
		t.Fatalf("SelectedAccount(round trip) error = %v", err)
	}
	if selected.Login() != login || selected.Password() != password || selected.Profile() != profile {
		t.Error("round trip changed one or more account values")
	}
	secondSerialization, err := SerializeDotenv(reloaded)
	if err != nil {
		t.Fatalf("SerializeDotenv(reloaded) error = %v", err)
	}
	if string(secondSerialization) != string(serialized) {
		t.Errorf("serialize/parse/serialize changed bytes:\n got %q\nwant %q", secondSerialization, serialized)
	}
}

func TestAccountAndDocumentFormattingRedactsValues(t *testing.T) {
	const sentinel = "sentinel-format-secret"
	account, err := NewAccount("operator", "sentinel-login", sentinel, "sentinel-profile")
	if err != nil {
		t.Fatalf("NewAccount() error = %v", err)
	}
	document := Document{accounts: []Account{account}}
	for _, formatted := range []string{
		fmt.Sprintf("%v", account), fmt.Sprintf("%+v", account), fmt.Sprintf("%#v", account), fmt.Sprintf("%s", account),
		fmt.Sprintf("%v", document), fmt.Sprintf("%+v", document), fmt.Sprintf("%#v", document),
	} {
		if strings.Contains(formatted, sentinel) || strings.Contains(formatted, "sentinel-login") || strings.Contains(formatted, "sentinel-profile") {
			t.Errorf("formatting leaked an account value: %q", formatted)
		}
	}
	for name, value := range map[string]any{"account": account, "document": document, "accounts": []Account{account}} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("json.Marshal(%s) error = %v", name, err)
		}
		if strings.Contains(string(encoded), sentinel) || strings.Contains(string(encoded), "sentinel-login") || strings.Contains(string(encoded), "sentinel-profile") {
			t.Errorf("JSON leaked %s values: %s", name, encoded)
		}
	}
}

func TestDotenvRejectsUnsupportedAccountControlCharacters(t *testing.T) {
	account, err := NewAccount("operator", "login", "password\x01value", "operator")
	if err != nil {
		t.Fatalf("NewAccount() error = %v", err)
	}
	document := Document{}
	if err := document.SetAccounts([]Account{account}); err == nil || !strings.Contains(err.Error(), "password") {
		t.Fatalf("SetAccounts() error = %v, want unsupported password control character", err)
	}
	if _, err := ParseDotenv([]byte("HERO_TEST_USERS=operator\nHERO_TEST_USER_OPERATOR_PASSWORD=\"bad\x01value\"")); err == nil {
		t.Fatal("ParseDotenv() accepted an unsupported control character")
	}
}
