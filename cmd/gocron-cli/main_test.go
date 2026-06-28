package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/urfave/cli"
)

func TestValuesFromSimpleYAMLUsesWebDefaults(t *testing.T) {
	values, err := valuesFromSimpleYAML([]byte(`
name: nightly-sync
spec: "0 0 2 * * *"
command: "php /data/app/index.php sync/run"
host_id: "1"
`))
	if err != nil {
		t.Fatal(err)
	}

	assertValue(t, values.Get("name"), "nightly-sync")
	assertValue(t, values.Get("command"), "php /data/app/index.php sync/run")
	assertValue(t, values.Get("level"), "1")
	assertValue(t, values.Get("dependency_status"), "1")
	assertValue(t, values.Get("protocol"), "2")
	assertValue(t, values.Get("http_method"), "1")
	assertValue(t, values.Get("timeout"), "0")
	assertValue(t, values.Get("multi"), "2")
	assertValue(t, values.Get("notify_status"), "1")
	assertValue(t, values.Get("notify_type"), "2")
	assertValue(t, values.Get("retry_times"), "0")
	assertValue(t, values.Get("retry_interval"), "0")
	assertValue(t, values.Get("status"), "1")
}

func TestValuesFromSimpleYAMLOverridesWebDefaults(t *testing.T) {
	values, err := valuesFromSimpleYAML([]byte(`
name: healthcheck
level: 2
protocol: 1
http_method: 2
command: "https://example.com/health"
status: 0
`))
	if err != nil {
		t.Fatal(err)
	}

	assertValue(t, values.Get("level"), "2")
	assertValue(t, values.Get("protocol"), "1")
	assertValue(t, values.Get("http_method"), "2")
	assertValue(t, values.Get("status"), "0")
}

func TestTaskListValuesSupportsPaginationAliasesAndFilters(t *testing.T) {
	set := flag.NewFlagSet("task-list", flag.ContinueOnError)
	for _, item := range taskListFlags() {
		item.Apply(set)
	}
	args := []string{
		"--page", "2",
		"--pagesize", "50",
		"--name", "sync",
		"--host-id", "3",
		"--protocol", "2",
		"--tag", "ops",
		"--command", "php",
		"--status", "1",
	}
	if err := set.Parse(args); err != nil {
		t.Fatal(err)
	}

	values := taskListValues(cli.NewContext(cli.NewApp(), set, nil))
	assertValue(t, values.Get("page"), "2")
	assertValue(t, values.Get("page_size"), "50")
	assertValue(t, values.Get("name"), "sync")
	assertValue(t, values.Get("host_id"), "3")
	assertValue(t, values.Get("protocol"), "2")
	assertValue(t, values.Get("tag"), "ops")
	assertValue(t, values.Get("command"), "php")
	assertValue(t, values.Get("status"), "1")
}

func TestTaskListValuesPrefersPageSizeOverPagesize(t *testing.T) {
	set := flag.NewFlagSet("task-list", flag.ContinueOnError)
	for _, item := range taskListFlags() {
		item.Apply(set)
	}
	if err := set.Parse([]string{"--page-size", "20", "--pagesize", "50"}); err != nil {
		t.Fatal(err)
	}

	values := taskListValues(cli.NewContext(cli.NewApp(), set, nil))
	assertValue(t, values.Get("page_size"), "20")
}

func TestGetRequestIncludesQueryValues(t *testing.T) {
	values := url.Values{}
	values.Set("page", "2")
	values.Set("page_size", "50")

	endpoint := endpointWithQuery("https://gocron.example.com/api/agent/v1/tasks", values)
	assertValue(t, endpoint, "https://gocron.example.com/api/agent/v1/tasks?page=2&page_size=50")
}

func TestTokenExpiresAtUsesExpiresIn(t *testing.T) {
	now := time.Unix(1000, 0)

	expiresAt := tokenExpiresAt(now, 1800)

	if expiresAt != 2800 {
		t.Fatalf("expected expires_at 2800, got %d", expiresAt)
	}
}

func TestRefreshSkipsNetworkWhenConfigAlreadyHasFreshToken(t *testing.T) {
	dir, err := ioutil.TempDir("", "gocron-cli-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	restore := useTestConfigPath(filepath.Join(dir, "config.json"))
	defer restore()

	now := time.Unix(1000, 0)
	cfg := &config{Profiles: map[string]*profile{
		defaultProfile: {
			Server:               "https://gocron.example.com",
			DeviceId:             "device-1",
			AccessToken:          "fresh-access",
			RefreshToken:         "fresh-refresh",
			AccessTokenExpiresAt: tokenExpiresAt(now, 1800),
		},
	}}
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	stale := &profile{
		Server:               "https://gocron.example.com",
		DeviceId:             "device-1",
		AccessToken:          "stale-access",
		RefreshToken:         "stale-refresh",
		AccessTokenExpiresAt: tokenExpiresAt(now, -1),
	}
	calls := 0
	client := func(*profile) (*tokenData, error) {
		calls++
		return nil, nil
	}

	if err := refreshProfile(defaultProfile, stale, now, client); err != nil {
		t.Fatal(err)
	}

	if calls != 0 {
		t.Fatalf("expected refresh network call to be skipped, got %d calls", calls)
	}
	assertValue(t, stale.AccessToken, "fresh-access")
	assertValue(t, stale.RefreshToken, "fresh-refresh")
}

func TestRefreshPersistsExpiresAtAndRotatedTokens(t *testing.T) {
	dir, err := ioutil.TempDir("", "gocron-cli-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	restore := useTestConfigPath(filepath.Join(dir, "config.json"))
	defer restore()

	now := time.Unix(1000, 0)
	stale := &profile{
		Server:               "https://gocron.example.com",
		DeviceId:             "device-1",
		AccessToken:          "old-access",
		RefreshToken:         "old-refresh",
		AccessTokenExpiresAt: tokenExpiresAt(now, -1),
	}
	cfg := &config{Profiles: map[string]*profile{defaultProfile: stale}}
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	client := func(prof *profile) (*tokenData, error) {
		assertValue(t, prof.RefreshToken, "old-refresh")
		return &tokenData{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
			ExpiresIn:    1800,
		}, nil
	}

	if err := refreshProfile(defaultProfile, stale, now, client); err != nil {
		t.Fatal(err)
	}

	assertValue(t, stale.AccessToken, "new-access")
	assertValue(t, stale.RefreshToken, "new-refresh")
	if stale.AccessTokenExpiresAt != 2800 {
		t.Fatalf("expected in-memory expires_at 2800, got %d", stale.AccessTokenExpiresAt)
	}

	body, err := ioutil.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved config
	if err := json.Unmarshal(body, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Profiles[defaultProfile].AccessTokenExpiresAt != 2800 {
		t.Fatalf("expected saved expires_at 2800, got %d", saved.Profiles[defaultProfile].AccessTokenExpiresAt)
	}
}

func TestShouldRefreshAfterErrorOnlyAllowsAuthErrors(t *testing.T) {
	if shouldRefreshAfterError(apiError{Code: 1, Message: "表单验证失败"}) {
		t.Fatal("expected common failure to skip token refresh")
	}
	if !shouldRefreshAfterError(apiError{Code: 401, Message: "认证失败"}) {
		t.Fatal("expected auth failure to refresh token")
	}
	if !shouldRefreshAfterError(apiError{Code: 403, Message: "无权限访问"}) {
		t.Fatal("expected unauthorized failure to refresh token")
	}
}

func assertValue(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func useTestConfigPath(path string) func() {
	previous := configPathOverride
	configPathOverride = func() (string, error) {
		return path, nil
	}
	return func() {
		configPathOverride = previous
	}
}
